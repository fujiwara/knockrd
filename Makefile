build: cmd/knockrd/* *.go
	cd cmd/knockrd && go build

run: build
	./cmd/knockrd/knockrd
