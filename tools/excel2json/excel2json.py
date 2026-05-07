#!/usr/bin/env python3
"""Excel → JSON 변환 도구.

도메인별 .xlsx 파일을 읽어 .json으로 변환하고, 체크섬 포함 manifest.json을 생성한다.

사용법:
  python excel2json.py --target {server|client} [--input-dir DIR] [--output-dir DIR] [--version VERSION]

기본값:
  --input-dir  ./input
  --output-dir ./output/<version>
  --version    v%Y%m%d-%H%M (현재 시각 기반 자동 생성)

xlsx 포맷 (4행 헤더):
  1행: CLIENT 사용 여부 ('CLIENT'이면 클라가 사용)
  2행: SERVER 사용 여부 ('SERVER'이면 서버가 사용)
  3행: 데이터 타입 (예: INT, STRING). '(PK)' 접미가 있으면 PK 컬럼.
  4행: 컬럼명 (JSON 키로 사용)
  5행~: 데이터

  --target에 따라 해당 마커가 있는 컬럼만 추출한다.
  PK는 추출된 컬럼 중 정확히 1개여야 한다 (검증).

처리 대상:
  input/ 디렉토리의 모든 .xlsx 파일을 자동 처리한다.
  임시 파일(`~$`로 시작), 숨김 파일은 제외.

출력:
  output/<version>/<domain>.json     # 도메인별 JSON 배열
  output/<version>/manifest.json     # { version, files: [{name, checksum, pk}] }
"""

import argparse
import datetime
import hashlib
import json
import math
import sys
from pathlib import Path

try:
    import pandas as pd
except ImportError:
    print("pandas가 필요합니다. 'pip install pandas openpyxl'으로 설치하세요.", file=sys.stderr)
    sys.exit(1)


CLIENT_MARKER = "CLIENT"
SERVER_MARKER = "SERVER"
PK_MARKER = "(PK)"


def list_xlsx_files(input_dir: Path) -> list[Path]:
    """input 디렉토리의 모든 xlsx 파일을 반환한다.
    Excel 임시 파일(~$로 시작)과 숨김 파일은 제외한다.
    """
    files = []
    for p in sorted(input_dir.glob("*.xlsx")):
        if p.name.startswith("~$") or p.name.startswith("."):
            continue
        files.append(p)
    return files


def cell_str(v) -> str:
    """셀 값을 문자열로 정규화 (NaN/None은 빈 문자열)."""
    if v is None:
        return ""
    if isinstance(v, float) and math.isnan(v):
        return ""
    return str(v).strip()


def select_columns(
    df: pd.DataFrame, target: str
) -> tuple[list[int], list[str], list[str], str]:
    """target에 해당하는 컬럼 인덱스 + 컬럼명 + 타입 + PK 컬럼명을 반환한다.

    df는 header=None으로 읽은 raw DataFrame이며, 행 인덱스 0~3이 헤더이다.
    타입은 (PK) 접미사를 제거한 정규화된 형태로 반환 (예: 'INT', 'STRING').
    """
    if df.shape[0] < 5:
        raise ValueError("최소 4행 헤더 + 1행 데이터 필요")

    marker = SERVER_MARKER if target == "server" else CLIENT_MARKER
    marker_row_idx = 1 if target == "server" else 0

    indices: list[int] = []
    names: list[str] = []
    types: list[str] = []
    pk_names: list[str] = []

    for col_idx in range(df.shape[1]):
        if cell_str(df.iat[marker_row_idx, col_idx]).upper() != marker:
            continue
        col_name = cell_str(df.iat[3, col_idx])
        if not col_name:
            raise ValueError(f"컬럼 인덱스 {col_idx}: 컬럼명(4행)이 비어있음")
        type_cell = cell_str(df.iat[2, col_idx]).upper()
        is_pk = PK_MARKER in type_cell
        type_name = type_cell.replace(PK_MARKER, "").strip()

        indices.append(col_idx)
        names.append(col_name)
        types.append(type_name)
        if is_pk:
            pk_names.append(col_name)

    if not indices:
        raise ValueError(f"{target} 사용 컬럼이 없음")
    if len(pk_names) != 1:
        raise ValueError(
            f"PK는 정확히 1개여야 함 (현재 {len(pk_names)}개: {pk_names})"
        )

    return indices, names, types, pk_names[0]


INT_TYPES = {"INT", "INT8", "INT16", "INT32", "INT64", "UINT", "UINT8", "UINT16", "UINT32", "UINT64", "LONG"}
FLOAT_TYPES = {"FLOAT", "FLOAT32", "FLOAT64", "DOUBLE", "REAL"}
BOOL_TYPES = {"BOOL", "BOOLEAN"}
STRING_TYPES = {"STRING", "STR", "TEXT"}


def coerce_value(v, type_name: str):
    """타입에 맞는 기본값 적용 + 타입 캐스팅.

    빈 셀 (NaN/None): STRING→"", INT→0, FLOAT→0.0, BOOL→False, 그 외→None.
    값이 있을 경우: INT는 int, FLOAT는 float, BOOL은 bool로 캐스팅 (1.0 → 1).
    """
    is_empty = v is None or (isinstance(v, float) and math.isnan(v))

    if type_name in STRING_TYPES:
        return "" if is_empty else str(v)
    if type_name in INT_TYPES:
        if is_empty:
            return 0
        try:
            return int(v)
        except (TypeError, ValueError):
            return 0
    if type_name in FLOAT_TYPES:
        if is_empty:
            return 0.0
        try:
            return float(v)
        except (TypeError, ValueError):
            return 0.0
    if type_name in BOOL_TYPES:
        if is_empty:
            return False
        if isinstance(v, bool):
            return v
        s = str(v).strip().upper()
        return s in ("TRUE", "1", "Y", "YES")
    # 알 수 없는 타입: NaN은 None, 나머지는 그대로
    return None if is_empty else v


def to_records(xlsx_path: Path, target: str) -> tuple[list[dict], str]:
    """xlsx 파일을 dict 배열로 변환한다. (records, pk_column) 반환.

    target에 해당하는 컬럼만 추출. 4행은 헤더, 5행부터 데이터.
    빈 셀은 타입별 기본값으로 채움 (STRING→"", INT→0, FLOAT→0.0, BOOL→false).
    """
    df = pd.read_excel(xlsx_path, sheet_name=0, header=None)
    indices, names, types, pk = select_columns(df, target)

    data_df = df.iloc[4:, indices].copy()
    data_df.columns = names

    records: list[dict] = []
    for _, row in data_df.iterrows():
        rec = {}
        for name, type_name in zip(names, types):
            rec[name] = coerce_value(row[name], type_name)
        records.append(rec)
    return records, pk


def write_json(records: list[dict], out_path: Path) -> bytes:
    """records를 JSON으로 직렬화하여 파일에 쓰고, 바이트를 반환한다."""
    data = json.dumps(records, ensure_ascii=False, indent=2).encode("utf-8")
    out_path.write_bytes(data)
    return data


def sha256_hex(data: bytes) -> str:
    return "sha256:" + hashlib.sha256(data).hexdigest()


def build_manifest(
    version: str, target: str, file_entries: list[dict]
) -> dict:
    return {
        "version": version,
        "target": target,
        "files": file_entries,
    }


def main():
    parser = argparse.ArgumentParser(description="Excel → JSON 변환 도구")
    parser.add_argument(
        "--target",
        choices=["server", "client"],
        required=True,
        help="추출 대상 (CLIENT/SERVER 마커 행으로 컬럼 필터링)",
    )
    parser.add_argument("--input-dir", default="input", help="xlsx 입력 디렉토리")
    parser.add_argument("--output-dir", default="output", help="JSON 출력 루트 디렉토리")
    parser.add_argument(
        "--version",
        default=datetime.datetime.now().strftime("v%Y%m%d-%H%M"),
        help="버전 문자열 (출력 하위 폴더명)",
    )
    args = parser.parse_args()

    input_dir = Path(args.input_dir)
    output_dir = Path(args.output_dir) / args.version
    output_dir.mkdir(parents=True, exist_ok=True)

    if not input_dir.is_dir():
        print(f"input 디렉토리가 존재하지 않습니다: {input_dir}", file=sys.stderr)
        sys.exit(1)

    xlsx_files = list_xlsx_files(input_dir)
    if not xlsx_files:
        print(f"xlsx 파일이 없습니다: {input_dir}", file=sys.stderr)
        sys.exit(1)

    file_entries: list[dict] = []
    fail = 0

    for xlsx_path in xlsx_files:
        try:
            records, pk = to_records(xlsx_path, args.target)
        except Exception as e:
            print(f"err:  {xlsx_path}: {e}", file=sys.stderr)
            fail += 1
            continue

        out_name = f"{xlsx_path.stem}.json"
        out_path = output_dir / out_name
        data = write_json(records, out_path)
        checksum = sha256_hex(data)
        file_entries.append({"name": out_name, "checksum": checksum, "pk": pk})
        print(f"ok:   {xlsx_path} → {out_path} ({len(records)} rows, pk={pk})")

    if not file_entries:
        print("처리된 파일이 없습니다.", file=sys.stderr)
        sys.exit(1)

    manifest = build_manifest(args.version, args.target, file_entries)
    manifest_path = output_dir / "manifest.json"
    manifest_path.write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )
    print(f"ok:   manifest → {manifest_path}")
    print(f"\ntarget:  {args.target}")
    print(f"version: {args.version}")
    print(f"output:  {output_dir}")

    if fail > 0:
        sys.exit(1)


if __name__ == "__main__":
    main()
