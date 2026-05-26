.PHONY: help build run proto clean vet test client design design-struct admin-ui release \
        mig-up mig-up-game mig-up-shard mig-up-one \
        mig-down mig-down-game mig-down-shard \
        mig-status mig-baseline mig-new-game mig-new-shard

# `make` (인자 없이) 입력 시 help 표시.
.DEFAULT_GOAL := help

# ---------- 도움말 ----------
# 규칙:
#   1) `target: ## 설명` — help 에 노출 (한 줄 요약, 가능한 짧게)
#   2) `##@ 카테고리명`  — 섹션 헤더로 그룹핑
#   3) 추가 설명 / 사용 예는 타겟 위쪽 일반 주석으로 (#)
help: ## 이 도움말 표시
	@awk 'BEGIN { \
	    FS = ":.*?## "; \
	    printf "\nUsage: \033[1mmake \033[36m<target>\033[0m\n"; \
	  } \
	  /^##@ / { printf "\n\033[1m%s\033[0m\n", substr($$0, 5); next } \
	  /^[a-zA-Z_-]+:.*?## / { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 } \
	  END { print "" }' $(MAKEFILE_LIST)

##@ 빌드 & 실행

# admin UI 미빌드 시 placeholder 페이지가 embed 됨 — 운영 배포는 `make release` 사용.
build: ## 서버 바이너리 빌드 (gameserver)
	go build -o gameserver .

# Vite 출력이 internal/admin_ui/dist 로 직접 들어감.
admin-ui: ## 어드민 React SPA 빌드
	cd web/admin && npm install && npm run build

release: admin-ui build  ## 운영 배포용 — UI + Go 둘 다 빌드

# 시작 전 pending 마이그레이션 경고 표시 (auto-apply 안 함).
run: ## 서버 실행
	@go run ./cmd/migrator status 2>/dev/null | awk '/pending=[1-9]/ { found=1 } END { if (found) { print ""; print "⚠ pending 마이그레이션 있음. `make mig-up` 먼저 실행하세요. (계속 진행하려면 Ctrl-C 후 5 초 내 재실행)"; print "" } }'
	go run main.go

proto: ## .proto → pb/ 코드 재생성
	protoc --proto_path=proto --go_out=pb --go_opt=paths=source_relative proto/*.proto

vet: ## 정적 분석 (go vet ./...)
	go vet ./...

# 캐시 무시. MySQL/Redis 가 로컬에 떠 있어야 한다.
# 사용 예: make test
#         make test PKG=./internal/handler/account/...
#         make test RUN=TestLogin_Success
#         make test V=1
test: ## 테스트 실행 (PKG, RUN, V 변수 지원)
	go test $(or $(PKG),./...) -count=1 $(if $(RUN),-run $(RUN),) $(if $(V),-v,)

client: ## 인터랙티브 테스트 클라이언트 실행 (VERSION 변수)
	go run ./cmd/client -cv $(VERSION)

clean: ## 빌드 산출물 삭제
	rm -f gameserver

##@ 디자인 데이터

# Excel → JSON 변환 (디자인 데이터).
# 사용 예: make design VERSION=v1.0.5   (기본 target=server)
#         make design TARGET=client
design: ## Excel → JSON (TARGET, VERSION 변수)
	cd tools/excel2json && source .venv/bin/activate && python excel2json.py --target $(or $(TARGET),server) $(if $(VERSION),--version $(VERSION),)

design-struct: ## Excel → Go struct (서버 전용)
	cd tools/excel2struct && source .venv/bin/activate && python excel2struct.py

##@ DB 마이그레이션
# 디렉토리: sql/migrations/<db>/<시간>_init_<...>.sql                  ← baseline (루트)
#           sql/migrations/<db>/<버전>/<시간>_<작업자>_<코멘트>.sql    ← 릴리즈별 변경
# 컨벤션:
#  - 버전 형식: x.yy.zz (예: 2.01.02) — TB_VERSION 의 client_version 포맷
#  - 작업자: git config user.email 의 로컬 파트 자동 사용. --author 로 override 가능
#  - 한 번 머지된 파일은 수정하지 말고, 새 파일로 fix migration 작성

# 사용 예: make mig-new-game version=2.01.02 name=add_account_last_seen
mig-new-game: ## 새 게임 DB 마이그레이션 파일 생성 (version=, name=)
	@if [ -z "$(version)" ] || [ -z "$(name)" ]; then \
	  echo "usage: make mig-new-game version=X.YY.ZZ name=foo_bar"; exit 1; \
	fi
	go run ./cmd/migrator create --db game --version $(version) --name $(name)

mig-new-shard: ## 새 샤드 DB 마이그레이션 파일 생성 (version=, name=)
	@if [ -z "$(version)" ] || [ -z "$(name)" ]; then \
	  echo "usage: make mig-new-shard version=X.YY.ZZ name=foo_bar"; exit 1; \
	fi
	go run ./cmd/migrator create --db shard --version $(version) --name $(name)

mig-up: ## pending 모두 적용 (game + 모든 shard)
	go run ./cmd/migrator up

mig-up-game: ## 게임 DB 만 적용
	go run ./cmd/migrator up --db game

mig-up-shard: ## 샤드 DB 만 적용
	go run ./cmd/migrator up --db shard

mig-up-one: ## pending 중 1 개만 적용 (디버깅용)
	go run ./cmd/migrator up-by-one

mig-down: ## 직전 적용된 마이그레이션 1 개 롤백 — game + 모든 shard (로컬 dev 권장)
	go run ./cmd/migrator down

mig-down-game: ## 게임 DB 만 롤백
	go run ./cmd/migrator down --db game

mig-down-shard: ## 샤드 DB 만 롤백
	go run ./cmd/migrator down --db shard

mig-status: ## 현재 적용 상태 출력
	go run ./cmd/migrator status

# 기존 sql/ 로 셋업된 DB 를 goose 관리 체계로 편입할 때 1 회 호출.
mig-baseline: ## 모든 마이그레이션을 실행 없이 적용 완료로 마킹
	go run ./cmd/migrator baseline
