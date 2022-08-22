all:
	./build.sh

linux:
	./build.sh linux

fmt:
	gofmt -s -w .
	goimports -w .

test: 
	go test -v -p 1 $$(go list ./... | grep -v /examples | grep -v tests/websocket-perf)

bench: 
	go test -bench=. $$(go list ./... | grep -v /examples | grep -v tests/websocket-perf)

.PHONY: all linux fmt test
