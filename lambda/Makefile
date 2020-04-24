knockrd: ../cmd/knockrd/*.go ../*.go ../go.*
	GOARCH=amd64 GOOS=linux make -C .. build
	install ../cmd/knockrd/knockrd .

clean:
	rm -f knockrd

deploy-http: knockrd
	lambroll --function function.http.json --log-level debug deploy

deploy-stream: knockrd
	lambroll --function function.stream.json --log-level debug deploy
