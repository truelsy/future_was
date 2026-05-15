.PHONY: build run proto clean vet test client design design-struct admin-ui release

# 서버 빌드 (admin UI 미빌드 시 placeholder 페이지가 embed 됨 — 운영 배포는 `make release` 사용)
build:
	go build -o server .

# 어드민 React SPA 빌드 (Vite 출력이 internal/admin_ui/dist 로 직접 들어감)
admin-ui:
	cd web/admin && npm install && npm run build

# 운영 배포용 — UI + Go 둘 다 빌드
release: admin-ui build

# 서버 실행
run:
	go run main.go

# protobuf 코드 생성
proto:
	protoc --proto_path=proto --go_out=pb --go_opt=paths=source_relative proto/*.proto

# 정적 분석
vet:
	go vet ./...

# 테스트 실행 (캐시 무시). MySQL/Redis가 로컬에 떠 있어야 한다.
# 사용 예: make test
#         make test PKG=./internal/handler/account/...
#         make test RUN=TestLogin_Success
test:
	go test $(or $(PKG),./...) -count=1 $(if $(RUN),-run $(RUN),) $(if $(V),-v,)

# 테스트 클라이언트 실행
client:
	go run ./cmd/client -cv $(VERSION)

# 빌드 산출물 삭제
clean:
	rm -f server

# 디자인 데이터 변환 (Excel → JSON)
# 사용 예: make design VERSION=v1.0.5  (기본 target=server)
#         make design TARGET=client
design:
	cd tools/excel2json && source .venv/bin/activate && python excel2json.py --target $(or $(TARGET),server) $(if $(VERSION),--version $(VERSION),)

# 디자인 struct 코드 생성 (Excel → Go struct)
design-struct:
	cd tools/excel2struct && source .venv/bin/activate && python excel2struct.py
