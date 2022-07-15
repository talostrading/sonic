package sonic

import (
	"bytes"
	"math/rand"
	"testing"
	"time"
)

func TestBytesBufferReads(t *testing.T) {
	b := NewBytesBuffer()

	// prepare the buffer for a read
	b.Prepare(512)
	if len(b.data) != 512 {
		t.Fatal("invalid length")
	}

	b.Prepare(1024)
	if len(b.data) != 1024 {
		t.Fatal("invalid length")
	}

	// read something
	msg := []byte("hello")
	rd := bytes.NewReader(msg)
	n, err := b.ReadFrom(rd)
	if err != nil {
		t.Fatal(err)
	}

	if b.ri != 0 && b.wi != int(n) {
		t.Fatalf("invalid read/write areas")
	}

	if b.ReadLen() != 0 {
		t.Fatalf("invalid read area length")
	}

	// make the read data available to the caller
	b.Commit(int(n))
	if string(b.Data()) != string(msg) || b.ReadLen() != len(msg) {
		t.Fatal("invalid data")
	}

	// consume some part of the data
	b.Consume(1)
	if string(b.Data()) != string(msg[1:]) || b.ReadLen() != len(msg[1:]) {
		t.Fatal("invalid data")
	}
	if b.ri != 4 || b.wi != 4 {
		t.Fatal("invalid read/write areas")
	}

	// write some more into the buffer
	msg2 := []byte("sonic")
	rd = bytes.NewReader(msg2)
	n, err = b.ReadFrom(rd)
	if err != nil {
		t.Fatal(err)
	}
	if b.ri != 4 || b.wi != 4+int(n) {
		t.Fatalf("invalid read/write areas")
	}

	// commit more than needed
	msg = append(msg[1:], msg2...)
	b.Commit(100)
	if string(b.Data()) != string(append(msg)) {
		t.Fatal("invalid data")
	}

	// consume more than needed
	b.Consume(100)
	if string(b.Data()) != "" || b.ReadLen() != 0 {
		t.Fatal("invalid data")
	}
	if b.ri != 0 || b.wi != 0 {
		t.Fatal("invalid read/write areas")
	}
}

func BenchmarkBytesBuffer(b *testing.B) {
	var letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	genRand := func(b []byte) []byte {
		for i := range b {
			b[i] = letters[rand.Intn(len(b))]
		}
		return b
	}

	rand.Seed(time.Now().UnixNano())

	n := 10
	msg := make([]byte, n)
	msg = genRand(msg)
	buf := NewBytesBuffer()

	for i := 0; i < b.N; i++ {
		buf.Write(msg)
		buf.Commit(n)
		buf.Consume(n)
	}

	b.ReportAllocs()
}
