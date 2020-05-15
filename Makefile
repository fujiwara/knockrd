.PHONY: test run clean prepare

prepare:
	go get github.com/rakyll/statik
	statik -src=./public

build: prepare cmd/knockrd/* *.go
	cd cmd/knockrd && go build

run: build
	./cmd/knockrd/knockrd

test: prepare
	go clean -testcache
	go test -v -race ./...

clean:
	rm -f statik/*

bump/patch:
	gobump patch -w cmd/knockrd
