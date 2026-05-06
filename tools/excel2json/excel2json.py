#!/usr/bin/env python3
"""Excel → JSON 변환 도구.

도메인별 .xlsx 파일을 읽어 .json으로 변환하고, 체크섬 포함 manifest.json을 생성한다.

사용법:
  python excel2json.py [--input-dir DIR] [--output-dir DIR] [--version VERSION]

기본값:
  --input-dir  ./input
  --output-dir ./output/<version>
  --version    YYYYMMDD-HHMM (현재 시각 기반 자동 생성)

처리 대상:
  input/ 디렉토리의 모든 .xlsx 파일을 자동으로 처리한다.
  예: input/cards.xlsx → output/<version>/cards.json

각 .xlsx 파일은 단일 시트 사용. 첫 행은 헤더(컬럼명).
컬럼명은 JSON snake_case 키로 그대로 사용된다.
임시 파일(`~$`로 시작)은 자동으로 제외된다.

출력:
  output/<version>/<domain>.json     # 도메인별 JSON 배열
  output/<version>/manifest.json     # { version, files: [{name, checksum}] }
"""

import argparse
import datetime
import hashlib
import json
import sys
from pathlib import Path

try:
    import pandas as pd
except ImportError:
    print("pandas가 필요합니다. 'pip install pandas openpyxl'으로 설치하세요.", file=sys.stderr)
    sys.exit(1)


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


def to_records(xlsx_path: Path) -> list[dict]:
    """xlsx 파일의 첫 시트를 dict 배열로 변환한다."""
    df = pd.read_excel(xlsx_path, sheet_name=0)
    # NaN을 None으로 처리하여 JSON null로 직렬화
    df = df.where(pd.notnull(df), None)
    return df.to_dict(orient="records")


def write_json(records: list[dict], out_path: Path) -> bytes:
    """records를 JSON으로 직렬화하여 파일에 쓰고, 바이트를 반환한다."""
    data = json.dumps(records, ensure_ascii=False, indent=2).encode("utf-8")
    out_path.write_bytes(data)
    return data


def sha256_hex(data: bytes) -> str:
    return "sha256:" + hashlib.sha256(data).hexdigest()


def build_manifest(version: str, file_checksums: list[tuple[str, str]]) -> dict:
    return {
        "version": version,
        "files": [{"name": name, "checksum": checksum} for name, checksum in file_checksums],
    }


def main():
    parser = argparse.ArgumentParser(description="Excel → JSON 변환 도구")
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

    file_checksums: list[tuple[str, str]] = []

    for xlsx_path in xlsx_files:
        records = to_records(xlsx_path)
        out_name = f"{xlsx_path.stem}.json"
        out_path = output_dir / out_name
        data = write_json(records, out_path)
        checksum = sha256_hex(data)
        file_checksums.append((out_name, checksum))
        print(f"ok:   {xlsx_path} → {out_path} ({len(records)} rows)")

    manifest = build_manifest(args.version, file_checksums)
    manifest_path = output_dir / "manifest.json"
    manifest_path.write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )
    print(f"ok:   manifest → {manifest_path}")
    print(f"\nversion: {args.version}")
    print(f"output:  {output_dir}")


if __name__ == "__main__":
    main()
