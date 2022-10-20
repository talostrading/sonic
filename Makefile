all:
	./build.sh

linux:
	./build.sh linux

fmt:
	gofmt -s -w .
	goimports -w .

test: 
	GODEBUG=asyncpreemptoff=1 go test -v -p 1 $$(go list ./... | grep -v '/examples\|/tests\|/temp\|/sonicopts\|/sonicerrors')

bench: 
	GODEBUG=asyncpreemptoff=1 go test -bench=. $$(go list ./... | grep -v '/examples\|/tests\|/temp\|/sonicopts\|/sonicerrors')

.PHONY: all linux fmt test
