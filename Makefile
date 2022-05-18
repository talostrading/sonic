all:
	./build.sh

fmt:
	gofmt -s -w .
	goimports -w .

.PHONY: fmt
