.PHONY: build run proto clean vet client

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
	go run ./cmd/client

# 빌드 산출물 삭제
clean:
	rm -f server
