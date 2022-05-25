all:
	./build.sh

fmt:
	gofmt -s -w .
	goimports -w .

test: 
	go test -v $$(go list ./... | grep -v /examples)

.PHONY: all fmt test
