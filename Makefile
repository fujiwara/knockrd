.PHONY: test run clean statik

build: statik cmd/knockrd/* *.go go.mod go.sum
	cd cmd/knockrd && go build

statik: public/*
	go get github.com/rakyll/statik
	statik -src=./public

run: build
	./cmd/knockrd/knockrd

test:
	go clean -testcache
	go test -v -race ./...

clean:
	rm -f statik/*

bump/patch:
	gobump patch -w cmd/knockrd
