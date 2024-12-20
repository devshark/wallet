.PHONY: vendor build test short-test run-server run-build lint

vendor:
	go mod tidy && go mod vendor

build: vendor
	CGO_ENABLED=0 go build  -ldflags \
		"-w -s" \
		-o build/http \
		-tags netgo \
		-a ./app/cmd/

test:
	go test -v ./... -race -cover -coverprofile=coverage.txt

short-test: # excludes tests with external dependencies
	go test -v ./... -short -race -cover -coverprofile=coverage.txt

run-server:
	POSTGRES_HOST=localhost \
	POSTGRES_PORT=5433 \
	POSTGRES_USER=postgres \
	POSTGRES_PASSWORD=postgres \
	POSTGRES_DATABASE=postgres \
	REDIS_ADDRESS=localhost:6389 \
	PORT=8080 go run app/cmd/main.go

run-build: build
	POSTGRES_HOST=localhost \
	POSTGRES_PORT=5433 \
	POSTGRES_USER=postgres \
	POSTGRES_PASSWORD=postgres \
	POSTGRES_DATABASE=postgres \
	REDIS_ADDRESS=localhost:6389 \
	PORT=8080 ./build/http

lint:
	golangci-lint run
