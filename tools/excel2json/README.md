# excel2json

기획자가 작성한 도메인별 Excel 파일을 게임 서버용 JSON으로 변환하는 도구.

## 디렉토리

```
tools/excel2json/
├── excel2json.py        # 변환 스크립트
├── requirements.txt     # Python 의존성
├── input/               # xlsx 입력
│   ├── cards.xlsx
│   ├── skills.xlsx
│   └── items.xlsx
└── output/              # JSON 출력 (버전별 폴더)
    └── v20260504-1430/
        ├── cards.json
        ├── skills.json
        ├── items.json
        └── manifest.json
```

## 설치

```bash
cd tools/excel2json
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

## Excel 작성 규칙

- `input/` 디렉토리의 **모든 `.xlsx` 파일이 자동 처리됨** (도메인별 개별 파일)
- 파일명이 출력 JSON명이 됨: `cards.xlsx` → `cards.json`
- 각 파일의 **첫 시트** 사용
- 첫 행은 **헤더(컬럼명)**, JSON 키로 그대로 사용 (snake_case 권장)
- 빈 셀은 `null`로 변환됨
- Excel 임시 파일(`~$`로 시작)과 숨김 파일은 자동 제외

예: `cards.xlsx`

| card_id | name | base_atk | base_def |
|---------|------|----------|----------|
| 1001    | 강타자 | 100 | 50 |
| 1002    | 투수 | 80 | 60 |

## 사용

```bash
# 자동 버전 (현재 시각 기반)
python excel2json.py

# 명시적 버전
python excel2json.py --version v1.0.5

# 입출력 경로 지정
python excel2json.py --input-dir /path/to/xlsx --output-dir /path/to/out
```

## 출력 예시

`output/v1.0.5/cards.json`
```json
[
  {"card_id": 1001, "name": "강타자", "base_atk": 100, "base_def": 50}
]
```

`output/v1.0.5/manifest.json`
```json
{
  "version": "v1.0.5",
  "files": [
    {"name": "cards.json", "checksum": "sha256:abc..."},
    {"name": "skills.json", "checksum": "sha256:def..."}
  ]
}
```

## 다음 단계

생성된 `output/<version>/` 폴더 전체를 CDN의 `/design/<version>/` 경로에 업로드한 후,
운영자가 게임 서버에 reload API 호출:

```bash
curl -X POST "http://server:8089/admin/design/reload?version=v1.0.5"
```

## 새 도메인 추가

`input/` 디렉토리에 새 `.xlsx` 파일만 추가하면 자동으로 처리됨. 코드 수정 불필요.
