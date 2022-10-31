package websocket

import (
	"crypto/sha1"
	"testing"
)

func TestMakeServerResponseKey(t *testing.T) {
	key := MakeServerResponseKey(sha1.New(), []byte("dGhlIHNhbXBsZSBub25jZQ=="))
	if key != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatal("invalid server response key")
	}
}
