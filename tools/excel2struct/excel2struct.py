#!/usr/bin/env python3
"""Excel → Go struct 코드 생성 도구.

input 디렉토리의 xlsx 파일들을 읽어 디자인 데이터용 Go struct를 자동 생성한다.

사용법:
  python excel2struct.py [--input-dir DIR] [--output-dir DIR] [--package PKG]

기본값:
  --input-dir   ./input
  --output-dir  ./output
  --package     schema

xlsx 포맷 (4행 헤더, 서버 전용):
  1행: CLIENT 사용 여부 (사용 안 함)
  2행: SERVER 사용 여부 ('SERVER'이면 서버가 사용 → struct 필드로 추출)
  3행: 데이터 타입 (예: INT, STRING). '(PK)' 접미가 있으면 PK 컬럼.
  4행: 컬럼명 (Go 필드명/JSON 태그로 사용)
  5행~: 데이터

  PK는 추출된 컬럼 중 정확히 1개여야 한다 (검증).

규칙:
  - 파일명(stem)을 PascalCase로 변환 후 'Design' 접미사 부여
    cards.xlsx → CardsDesign
    BAT_DATA.xlsx → BatDataDesign
  - 컬럼명을 PascalCase Go 필드명으로 변환
  - 숫자로 시작하는 컬럼은 'F' prefix (2B → F2B)
  - JSON 태그는 원본 컬럼명 그대로
  - 임시 파일(~$로 시작), 숨김 파일은 제외

타입 매핑 (Excel → Go):
  STRING/STR/TEXT             → string
  INT/INT32                   → int32
  INT64/LONG                  → int64
  UINT/UINT32                 → uint32
  UINT64                      → uint64
  FLOAT/FLOAT32               → float32
  FLOAT64/DOUBLE/REAL         → float64
  BOOL/BOOLEAN                → bool
"""

import argparse
import math
import re
import sys
from pathlib import Path

try:
    import pandas as pd
except ImportError:
    print("pandas가 필요합니다. 'pip install pandas openpyxl'으로 설치하세요.", file=sys.stderr)
    sys.exit(1)


SERVER_MARKER = "SERVER"
PK_MARKER = "(PK)"

TYPE_MAP = {
    "STRING": "string", "STR": "string", "TEXT": "string",
    "INT": "int32", "INT32": "int32",
    "INT64": "int64", "LONG": "int64",
    "INT8": "int8", "INT16": "int16",
    "UINT": "uint32", "UINT32": "uint32",
    "UINT64": "uint64",
    "UINT8": "uint8", "UINT16": "uint16",
    "FLOAT": "float32", "FLOAT32": "float32",
    "FLOAT64": "float64", "DOUBLE": "float64", "REAL": "float64",
    "BOOL": "bool", "BOOLEAN": "bool",
}


def list_xlsx_files(input_dir: Path) -> list[Path]:
    """input 디렉토리의 모든 xlsx 파일을 반환한다 (임시/숨김 제외)."""
    files = []
    for p in sorted(input_dir.glob("*.xlsx")):
        if p.name.startswith("~$") or p.name.startswith("."):
            continue
        files.append(p)
    return files


def to_pascal_case(name: str) -> str:
    """snake_case / kebab-case / UPPER_SNAKE → PascalCase."""
    parts = re.split(r"[_\-\s]+", name)
    return "".join(p.capitalize() for p in parts if p)


def to_go_field_name(col: str) -> str:
    """컬럼명을 Go 공개 필드명으로 변환한다.

    숫자로 시작하는 컬럼(예: 2B, 3B)은 Go 식별자 규칙상 그대로 사용할 수 없으므로
    'F' (Field) prefix를 붙인다. (2B → F2B, 3B → F3B)
    """
    pascal = to_pascal_case(col)
    if pascal and pascal[0].isdigit():
        pascal = "F" + pascal
    return pascal


def to_snake_case_filename(pascal: str) -> str:
    """PascalCase → snake_case (출력 파일명용)."""
    s = re.sub(r"(.)([A-Z][a-z]+)", r"\1_\2", pascal)
    s = re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", s)
    return s.lower()


def cell_str(v) -> str:
    """셀 값을 문자열로 정규화 (NaN/None은 빈 문자열)."""
    if v is None:
        return ""
    if isinstance(v, float) and math.isnan(v):
        return ""
    return str(v).strip()


def go_type_for(type_name: str) -> str:
    """Excel 타입 문자열 → Go 타입. 미지원 타입은 string으로 fallback."""
    return TYPE_MAP.get(type_name.upper(), "string")


def derive_type_name(stem: str) -> str:
    """파일명 stem → Go 타입명 (PascalCase + 'Design' 접미사)."""
    base = to_pascal_case(stem)
    if not base.endswith("Design"):
        base += "Design"
    return base


def select_columns(df: pd.DataFrame) -> tuple[list[str], list[str], str]:
    """SERVER 마커가 있는 컬럼만 추출하여 (컬럼명 리스트, Go 타입 리스트, PK 컬럼명) 반환.

    df는 header=None으로 읽은 raw DataFrame이며, 행 인덱스 0~3이 헤더이다.
    """
    if df.shape[0] < 5:
        raise ValueError("최소 4행 헤더 + 1행 데이터 필요")

    names: list[str] = []
    types: list[str] = []
    pk_names: list[str] = []

    for col_idx in range(df.shape[1]):
        if cell_str(df.iat[1, col_idx]).upper() != SERVER_MARKER:
            continue
        col_name = cell_str(df.iat[3, col_idx])
        if not col_name:
            raise ValueError(f"컬럼 인덱스 {col_idx}: 컬럼명(4행)이 비어있음")
        type_cell = cell_str(df.iat[2, col_idx]).upper()
        is_pk = PK_MARKER in type_cell
        type_name = type_cell.replace(PK_MARKER, "").strip()

        names.append(col_name)
        types.append(go_type_for(type_name))
        if is_pk:
            pk_names.append(col_name)

    if not names:
        raise ValueError("SERVER 사용 컬럼이 없음")
    if len(pk_names) != 1:
        raise ValueError(
            f"PK는 정확히 1개여야 함 (현재 {len(pk_names)}개: {pk_names})"
        )

    return names, types, pk_names[0]


def render_struct(type_name: str, columns: list[tuple[str, str]], pk_column: str) -> str:
    """Go struct 코드 문자열을 생성한다.

    columns: [(원본_컬럼명, go_타입), ...]
    pk_column: PK 컬럼명 (주석 표시용)
    """
    field_specs = []
    max_field_len = 0
    max_type_len = 0
    for col, gotype in columns:
        field = to_go_field_name(col)
        field_specs.append((field, gotype, col))
        max_field_len = max(max_field_len, len(field))
        max_type_len = max(max_type_len, len(gotype))

    lines = [f"type {type_name} struct {{"]
    for field, gotype, col in field_specs:
        pad_field = " " * (max_field_len - len(field) + 1)
        pad_type = " " * (max_type_len - len(gotype) + 1)
        comment = "  // PK" if col == pk_column else ""
        lines.append(f"\t{field}{pad_field}{gotype}{pad_type}`json:\"{col}\"`{comment}")
    lines.append("}")
    return "\n".join(lines)


def render_file(package: str, type_name: str, struct_body: str, source_file: str) -> str:
    """완성된 .go 파일 문자열을 생성한다."""
    return (
        f"// Code generated by excel2struct from {source_file}. DO NOT EDIT.\n"
        f"package {package}\n\n"
        f"{struct_body}\n"
    )


def process(xlsx_path: Path, output_dir: Path, package: str) -> Path:
    """xlsx 한 파일을 처리하여 .go 파일을 생성한다. 출력 경로 반환."""
    df = pd.read_excel(xlsx_path, sheet_name=0, header=None)
    names, types, pk_col = select_columns(df)

    columns = list(zip(names, types))

    type_name = derive_type_name(xlsx_path.stem)
    struct_body = render_struct(type_name, columns, pk_col)
    content = render_file(package, type_name, struct_body, xlsx_path.name)

    out_path = output_dir / f"{to_snake_case_filename(type_name)}.go"
    out_path.write_text(content, encoding="utf-8")
    return out_path


def main():
    parser = argparse.ArgumentParser(description="Excel → Go struct 코드 생성 도구")
    parser.add_argument("--input-dir", default="input", help="xlsx 입력 디렉토리")
    parser.add_argument("--output-dir", default="output", help=".go 출력 디렉토리")
    parser.add_argument("--package", default="schema", help="생성된 파일의 Go 패키지명")
    args = parser.parse_args()

    input_dir = Path(args.input_dir)
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    if not input_dir.is_dir():
        print(f"input 디렉토리가 존재하지 않습니다: {input_dir}", file=sys.stderr)
        sys.exit(1)

    xlsx_files = list_xlsx_files(input_dir)
    if not xlsx_files:
        print(f"xlsx 파일이 없습니다: {input_dir}", file=sys.stderr)
        sys.exit(1)

    fail = 0
    for xlsx_path in xlsx_files:
        try:
            out_path = process(xlsx_path, output_dir, args.package)
            print(f"ok:  {xlsx_path} → {out_path}")
        except Exception as e:
            print(f"err: {xlsx_path}: {e}", file=sys.stderr)
            fail += 1

    if fail > 0:
        sys.exit(1)


if __name__ == "__main__":
    main()
