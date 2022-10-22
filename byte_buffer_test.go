package sonic

import (
	"bufio"
	"bytes"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/talostrading/sonic/sonicerrors"
)

func TestByteBuffer_Reserve(t *testing.T) {
	b := NewByteBuffer()

	b.Reserve(512)
	if b.Cap() != 512 {
		t.Fatal("should have not reserved")
	}
	if b.Reserved() < 512 {
		t.Fatal("wrong reserved")
	}

	b.Reserve(1024)
	if b.Cap() < 1024 {
		t.Fatal("should have reserved")
	}

	if b.Reserved() < 1024 {
		t.Fatal("wrong reserved")
	}

	temp := make([]byte, 1024)
	b.Write(temp)

	b.Commit(1024)

	if b.Reserved() > 1024 {
		t.Fatal("wrong reserved")
	}
}

func TestByteBuffer_Reads1(t *testing.T) {
	b := NewByteBuffer()

	b.Reserve(512)
	if b.Cap() < 512 {
		t.Fatal("invalid write area length")
	}

	b.Reserve(1024)
	if b.Cap() < 1024 {
		t.Fatal("invalid write area length")
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
	if given, expected := string(b.Data()), string(msg); given != expected {
		t.Fatalf("invalid data given=%s expected=%s", given, expected)
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

func TestByteBuffer_Reads2(t *testing.T) {
	b := NewByteBuffer()

	msg := []byte("hello")
	b.Write(msg)
	b.Commit(5)

	into := make([]byte, 10)
	n, err := b.Read(into)
	if err != nil {
		t.Fatal(err)
	}
	into = into[:n]

	if given, expected := string(into), string(msg); given != expected {
		t.Fatalf("invalid read given=%s expected=%s", given, expected)
	}
}

func TestByteBuffer_Writes(t *testing.T) {
	b := NewByteBuffer()

	n, err := b.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatal("wrong number of bytes written")
	}
	if b.ReadLen() != 0 {
		t.Fatal("wrong read area length")
	}
	if b.WriteLen() != 5 {
		t.Fatal("wrong write area length")
	}

	b.Commit(5)
	if b.ReadLen() != 5 {
		t.Fatal("wrong read area length")
	}
	if b.WriteLen() != 0 {
		t.Fatal("wrong write area length")
	}

	w := bufio.NewWriter(nil)
	nn, err := b.WriteTo(w)
	if err != nil {
		t.Fatal(err)
	}
	if nn != 5 {
		t.Fatal("wrong number of bytes written")
	}
	if b.ReadLen() != 0 { // the WriteTo consumed the data
		t.Fatal("wrong read area length")
	}
	if b.WriteLen() != 0 {
		t.Fatal("wrong write area length")
	}

	b.Consume(5)
	if b.ReadLen() != 0 {
		t.Fatal("wrong read area length")
	}
	if b.WriteLen() != 0 {
		t.Fatal("wrong write area length")
	}
}

func TestByteBuffer_PrepareRead(t *testing.T) {
	b := NewByteBuffer()

	b.Write([]byte("hello"))
	err := b.PrepareRead(3)
	if err != nil {
		t.Fatal(err)
	}
	if b.ReadLen() != 3 {
		t.Fatal("invalid read length")
	}
	if b.WriteLen() != 2 {
		t.Fatal("invalid write length")
	}

	err = b.PrepareRead(5)
	if b.ReadLen() != 5 {
		t.Fatal("invalid write length")
	}
	if b.WriteLen() != 0 {
		t.Fatal("invalid write length")
	}

	err = b.PrepareRead(10)
	if !errors.Is(err, sonicerrors.ErrNeedMore) {
		t.Fatal("should not be able to prepare read")
	}
}

func BenchmarkByteBuffer(b *testing.B) {
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
	buf := NewByteBuffer()

	for i := 0; i < b.N; i++ {
		buf.Write(msg)
		buf.Commit(n)
		buf.Consume(n)
	}

	b.ReportAllocs()
}
