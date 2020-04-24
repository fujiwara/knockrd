knockrd: ../cmd/knockrd/*.go ../*.go ../go.*
	GOARCH=amd64 GOOS=linux make -C .. build
	install ../cmd/knockrd/knockrd .

clean:
	rm -f knockrd

deploy/%: knockrd
	KNOCKRD_RUN=$* lambroll --log-level debug deploy
	$(MAKE) logs/$*

logs/%:
	KNOCKRD_RUN=$* lambroll logs --follow --format=short
