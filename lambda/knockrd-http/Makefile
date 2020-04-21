knockrd:
	GOARCH=amd64 GOOS=linux make -C ../../ build
	install ../../cmd/knockrd/knockrd .

clean:
	rm -f knockrd

deploy: knockrd
	lambroll --log-level debug deploy
