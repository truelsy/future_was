.PHONY: build run proto clean vet client design design-struct

# 서버 빌드
build:
	go build -o server .

# 서버 실행
run:
	go run main.go

# protobuf 코드 생성
proto:
	protoc --proto_path=proto --go_out=pb --go_opt=paths=source_relative proto/*.proto

# 정적 분석
vet:
	go vet ./...

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
