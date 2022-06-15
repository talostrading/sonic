all:
	./build.sh

linux:
	./build.sh linux

fmt:
	gofmt -s -w .
	goimports -w .

test: 
	go test -v $$(go list ./... | grep -v /examples | grep -v tests/websocket-perf)

.PHONY: all linux fmt test
