#!/bin/bash
WEBSOCKET_INTEGRATION_TEST_DUR=5 go test -memprofile mem.prof -v -run=TestClientReadWrite .
go tool pprof mem.prof -- top10
