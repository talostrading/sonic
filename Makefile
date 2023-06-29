all: lint
	./build.sh

linux: lint
	./build.sh linux

fmt:
	gofmt -s -w .
	goimports -w .

lint:
	golangci-lint -j 4 run --fast --timeout=5m

gosec:
	gosec -fmt=sarif -out=results.sarif -exclude-dir=examples -exclude-dir=stress_test -exclude-dir=other -exclude-dir=docs -exclude-dir=tests -exclude-dir=benchmark ./...

test:
	GODEBUG=asyncpreemptoff=1 go test -v -p 1 $$(go list ./... | grep -v /examples | grep -v tests/websocket-perf)

bench:
	GODEBUG=asyncpreemptoff=1 go test -bench=Benchmark -run=^# $$(go list ./... | grep -v /examples | grep -v tests/websocket-perf)

.PHONY: all linux fmt lint gosec test bench
