.PHONY: vendor build test run-server run-client short-test

vendor:
	go mod tidy && go mod vendor

build: vendor
	CGO_ENABLED=0 go build  -ldflags \
		"-w -s" \
		-o build/http \
		-tags netgo \
		-a ./app/cmd/

test:
	go test -v ./... -race -cover

short-test:
	go test -v ./... -short -race -cover

run-server:
	POSTGRES_HOST=localhost \
	POSTGRES_PORT=5433 \
	POSTGRES_USER=postgres \
	POSTGRES_PASSWORD=postgres \
	POSTGRES_DATABASE=postgres go run app/cmd/main.go

run-build: build
	PUBLIC_NODE_URL=https://ethereum-rpc.publicnode.com/ \
	PORT=8080 \
	JOB_SCHEDULE=1s \
	USE_DATABASE=false ./build/http

run-client:
	PARSER_URL=http://localhost:8080 FETCH_FREQUENCY=10s SUBSCRIBE_ADDRESSES="0xdAC17F958D2ee523a2206206994597C13D831ec7,0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599" go run client/cmd/main.go
