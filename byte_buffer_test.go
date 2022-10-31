package sonic

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
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

	b.Consume(100)

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
		t.Fatal("invalid read length")
	}
	if b.WriteLen() != 0 {
		t.Fatal("invalid write length")
	}

	err = b.PrepareRead(10)
	if !errors.Is(err, sonicerrors.ErrNeedMore) {
		t.Fatal("should not be able to prepare read")
	}
}

func TestByteBuffer_PrepareReadSlice(t *testing.T) {
	b := NewByteBuffer()

	msg := []byte("hello.hello")

	b.Write(msg)
	err := b.PrepareReadSlice('.')
	if err != nil {
		t.Fatal(err)
	}
	if b.ReadLen() != len("hello.") {
		t.Fatal("invalid read length")
	}
	if b.WriteLen() != len("hello") {
		t.Fatal("invalid write length")
	}

	err = b.PrepareReadSlice('.')
	if !errors.Is(err, sonicerrors.ErrNeedMore) {
		t.Fatal("should have received ErrNeedMore")
	}
}

func TestByteBuffer_PrepareReadLineLF(t *testing.T) {
	b := NewByteBuffer()

	msg := []byte("hello.hello\n")

	b.Write(msg)
	err := b.PrepareReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if b.ReadLen() != len(msg) {
		t.Fatal("invalid read length")
	}
	if b.WriteLen() != 0 {
		t.Fatal("invalid write length")
	}

	err = b.PrepareReadLine()
	if !errors.Is(err, sonicerrors.ErrNeedMore) {
		t.Fatal("should have received ErrNeedMore")
	}
}

func TestByteBuffer_PrepareReadLineCLRF(t *testing.T) {
	b := NewByteBuffer()

	msg := []byte("hello.hello\r\n")

	b.Write(msg)
	err := b.PrepareReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if b.ReadLen() != len(msg) {
		t.Fatal("invalid read length")
	}
	if b.WriteLen() != 0 {
		t.Fatal("invalid write length")
	}

	err = b.PrepareReadLine()
	if !errors.Is(err, sonicerrors.ErrNeedMore) {
		t.Fatal("should have received ErrNeedMore")
	}
}

func TestByteBuffer_ReadSlice(t *testing.T) {
	b := NewByteBuffer()

	msg := []byte("hello.sonic")
	b.Write(msg)

	_, err := b.ReadSlice('z')
	if err != sonicerrors.ErrNeedMore {
		t.Fatal("should have received ErrNeedMore")
	}

	if b.ReadLen() != 0 {
		t.Fatal("read region should be empty")
	}

	err = b.PrepareReadSlice('.')
	if err != nil {
		t.Fatal(err)
	}

	s, err := b.ReadSlice('z')
	if err != sonicerrors.ErrNeedMore {
		t.Fatal("should have received ErrNeedMore")
	}
	if len(s) != 0 {
		t.Fatal("should have returned no slice")
	}

	s, err = b.ReadSlice('.')
	if err != nil {
		t.Fatal("should have read slice")
	}

	if string(s) != "hello" {
		t.Fatal("wrong slice")
	}

	if b.ReadLen() != 0 {
		t.Fatal("wrong read region")
	}
	if b.leftover != 0 {
		t.Fatal("should have no leftover")
	}

	b.Commit(100) // commit the rest
	s, err = b.ReadSlice('.')
	if err != sonicerrors.ErrNeedMore {
		t.Fatal("should have ErrNeedMore")
	}

	if len(s) != 0 {
		t.Fatal("no slice since no delim")
	}

	if b.leftover != 0 {
		t.Fatal("should have consumed everything")
	}
}

func TestByteBuffer_ReadLineLF(t *testing.T) {
	b := NewByteBuffer()

	msg := []byte("hello.hello\n")
	b.Write(msg)

	_, err := b.ReadLine()
	if err != sonicerrors.ErrNeedMore {
		t.Fatal("should have received ErrNeedMore")
	}

	err = b.PrepareReadLine()
	if err != nil {
		t.Fatal(err)
	}

	s, err := b.ReadLine()
	if err != nil {
		t.Fatal("should have read line")
	}

	if string(s) != string(bytes.TrimSpace(msg)) {
		t.Fatal("wrong slice")
	}
}

func TestByteBuffer_ReadLineCLRF(t *testing.T) {
	b := NewByteBuffer()

	msg := []byte("hello.hello\r\n")
	b.Write(msg)

	_, err := b.ReadLine()
	if err != sonicerrors.ErrNeedMore {
		t.Fatal("should have received ErrNeedMore")
	}

	err = b.PrepareReadLine()
	if err != nil {
		t.Fatal(err)
	}

	s, err := b.ReadLine()
	if err != nil {
		t.Fatal("should have read line")
	}

	if string(s) != string(bytes.TrimSpace(msg)) {
		t.Fatal("wrong slice")
	}

	// This should consume the line and one more byte, the LF newline.
	b.Consume(len(s))

	if b.WriteLen() != 0 {
		t.Fatal("write region should be empty")
	}
	if b.ReadLen() != 0 {
		t.Fatal("read region should be empty")
	}
}

func TestByteBuffer_ReadLineMulti(t *testing.T) {
	b := NewByteBuffer()

	msg := []byte("hello\nsonic\r\nagain\n")
	b.Write(msg)

	_, err := b.ReadLine()
	if err != sonicerrors.ErrNeedMore {
		t.Fatal("should have received ErrNeedMore")
	}

	err = b.PrepareReadLine()
	if err != nil {
		t.Fatal(err)
	}

	s, err := b.ReadLine()
	if err != nil {
		t.Fatal("should have read line")
	}
	if string(s) != "hello" {
		t.Fatal("wrong slice")
	}
	if b.leftover != 6 {
		t.Fatal("wrong leftover")
	}

	s, err = b.ReadLine()
	if err != sonicerrors.ErrNeedMore {
		t.Fatal("should have returned ErrNeedMore")
	}

	b.PrepareReadLine()

	s, err = b.ReadLine()
	if err != nil {
		t.Fatal("should have read line")
	}
	if string(s) != "sonic" {
		fmt.Println(string(s))
		t.Fatal("wrong slice")
	}
	if b.leftover != 7 {
		t.Fatal("wrong leftover")
	}

	b.PrepareReadLine()

	s, err = b.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if string(s) != "again" {
		t.Fatal("wrong slice")
	}
	if b.leftover != 6 {
		t.Fatal("wrong leftover")
	}

	s, err = b.ReadLine()
	if err != sonicerrors.ErrNeedMore {
		fmt.Println(err)
		t.Fatal("should have returned ErrNeedMore")
	}

	if b.WriteLen() != 0 {
		t.Fatal("write region should be empty")
	}
	if b.ReadLen() != 0 {
		t.Fatal("read region should be empty")
	}
}

func TestByteBuffer_CommitAll(t *testing.T) {
	b := NewByteBuffer()

	b.Write([]byte("all"))
	b.CommitAll()

	if len(b.Data()) != 3 || b.ReadLen() != 3 {
		t.Fatal("wrong read region length")
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
