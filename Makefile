.PHONY: test run clean statik

statik: public/*
	go get github.com/rakyll/statik
	statik -src=./public

build: cmd/knockrd/* *.go
	cd cmd/knockrd && go build

run: build
	./cmd/knockrd/knockrd

test:
	go clean -testcache
	go test -v -race ./...

clean:
	rm -f statik/*

bump/patch:
	gobump patch -w cmd/knockrd
