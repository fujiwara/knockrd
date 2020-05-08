.PHONY: test run

build: cmd/knockrd/* *.go
	cd cmd/knockrd && go build

run: build
	./cmd/knockrd/knockrd

test:
	go clean -testcache
	go test -v -race ./...
