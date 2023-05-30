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

func TestByteBufferReserve(t *testing.T) {
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

func TestByteBufferReads1(t *testing.T) {
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

func TestByteBufferReads2(t *testing.T) {
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

func TestByteBufferWrites(t *testing.T) {
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

func TestByteBufferPrepareRead(t *testing.T) {
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
	if err != nil {
		t.Fatal(err)
	}
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

func TestByteBufferClaim1(t *testing.T) {
	b := NewByteBuffer()

	// check setup
	if b.ReadLen() != 0 {
		t.Fatal("read area should have 0 length")
	}

	if b.WriteLen() != 0 {
		t.Fatal("write area should have 0 length")
	}

	if b.Reserved() != 512 {
		t.Fatal("should have 512 bytes reserved")
	}

	// claim multiple times and check
	b.Claim(func(b []byte) int {
		if len(b) != 512 {
			t.Fatal("should have provided 512 bytes")
		}
		for i := 0; i < 256; i++ {
			b[i] = 1
		}
		return 256
	})

	if b.Reserved() != 256 {
		t.Fatal("should have 256 bytes reserved")
	}

	if b.WriteLen() != 256 {
		t.Fatal("should have 256 bytes in the write area")
	}

	b.Commit(256)
	for i := 0; i < 256; i++ {
		if b.Data()[i] != 1 {
			t.Fatal("something wrong with claimed bytes")
		}
	}

	b.Claim(func(b []byte) int {
		if len(b) != 256 {
			t.Fatal("should have provided 256 bytes")
		}
		for i := 0; i < 256; i++ {
			b[i] = 2
		}
		return 256
	})

	if b.Reserved() != 0 {
		t.Fatal("should have 0 bytes reserved")
	}

	if b.WriteLen() != 256 {
		t.Fatal("should have 256 bytes in the write area")
	}

	b.Commit(256)
	// from before
	for i := 0; i < 256; i++ {
		if b.Data()[i] != 1 {
			t.Fatal("something wrong with claimed bytes")
		}
	}
	// the current ones
	for i := 256; i < 512; i++ {
		if b.Data()[i] != 2 {
			t.Fatal("something wrong with claimed bytes")
		}
	}

	if b.Reserved() != 0 {
		t.Fatal("should have 0 bytes reserved")
	}

	if b.WriteLen() != 0 {
		t.Fatal("should have 0 bytes in the write area")
	}

	if b.ReadLen() != 512 {
		t.Fatal("should have 512 bytes in the read area")
	}

	b.Consume(512)

	if b.Reserved() != 512 {
		t.Fatal("should have 512 bytes reserved")
	}

	if b.WriteLen() != 0 {
		t.Fatal("should have 0 bytes in the write area")
	}

	if b.ReadLen() != 0 {
		t.Fatal("should have 0 bytes in the read area")
	}
}

func TestByteBufferClaim2(t *testing.T) {
	b := NewByteBuffer()
	n := b.Reserved()

	// first claim should return the whole reserved area
	b.Claim(func(b []byte) int {
		if len(b) != n {
			t.Fatalf("wrong size on claimed slice expected=%d given=%d", n, len(b))
		}
		return n
	})

	// subsequent claims should return 0 bytes since we already claimed the whole reserved area
	for i := 0; i < 10; i++ {
		before := b.WriteLen()
		b.Claim(func(b []byte) int {
			if len(b) != 0 {
				t.Fatalf("wrong size on claimed slice expected=%d given=%d", 0, len(b))
			}
			return n // this should have no effect
		})
		after := b.WriteLen()
		if before != after {
			t.Fatal("write area should not be modified")
		}
	}
}

func TestByteBufferClaim3(t *testing.T) {
	b := NewByteBuffer()
	n := b.Reserved()
	n10 := n * 10

	before := b.WriteLen()
	b.Claim(func(b []byte) int {
		if len(b) != n {
			t.Fatalf("wrong size on claimed slice expected=%d given=%d", n, len(b))
		}
		return n10
	})
	after := b.WriteLen()
	if before != after {
		t.Fatal("the write index should not move past the reserved area on an over-claim")
	}
	if b.Reserved() != n {
		t.Fatal("the reserve area should be untouched as our Claim return is wrong")
	}
}

func TestByteBufferSaveAndDiscard1(t *testing.T) {
	b := NewByteBuffer()
	n, err := b.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatal("did not write 5")
	}

	b.Commit(2)

	if b.WriteLen() != 3 {
		t.Fatal("wrong write area")
	}
	if b.ReadLen() != 2 {
		t.Fatal("wrong read area")
	}
	if b.SaveLen() != 0 {
		t.Fatal("wrong save area")
	}

	slot := b.Save(1)

	if b.WriteLen() != 3 {
		t.Fatal("wrong write area")
	}
	if b.ReadLen() != 1 {
		t.Fatal("wrong read area")
	}
	if b.SaveLen() != 1 {
		t.Fatal("wrong save area")
	}
	if slot.Index != 0 || slot.Length != 1 {
		t.Fatalf("wrong slot %+v", slot)
	}
	if b := b.SavedSlot(slot); string(b) != "h" {
		t.Fatal("wrong slot")
	}
	if b := b.Saved(); string(b) != "h" {
		t.Fatal("wrong slot")
	}

	b.Consume(1)

	if b.WriteLen() != 3 {
		t.Fatal("wrong write area")
	}
	if b.ReadLen() != 0 {
		t.Fatal("wrong read area")
	}
	if b.SaveLen() != 1 {
		t.Fatal("wrong save area")
	}
	if slot.Index != 0 || slot.Length != 1 {
		t.Fatalf("wrong slot %+v", slot)
	}
	if b := b.SavedSlot(slot); string(b) != "h" {
		t.Fatal("wrong slot")
	}
	if b := b.Saved(); string(b) != "h" {
		t.Fatal("wrong slot")
	}

	b.Discard(slot)

	if b.WriteLen() != 3 {
		t.Fatal("wrong write area")
	}
	if b.ReadLen() != 0 {
		t.Fatal("wrong read area")
	}
	if b.SaveLen() != 0 {
		t.Fatal("wrong save area")
	}
}

func TestByteBufferSaveAndDiscard2(t *testing.T) {
	b := NewByteBuffer()
	n, err := b.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatal("did not write 5")
	}

	if b.WriteLen() != 5 {
		t.Fatal("wrong write area")
	}
	if b.ReadLen() != 0 {
		t.Fatal("wrong read area")
	}
	if b.SaveLen() != 0 {
		t.Fatal("wrong save area")
	}

	b.Save(10)

	if b.WriteLen() != 5 {
		t.Fatal("wrong write area")
	}
	if b.ReadLen() != 0 {
		t.Fatal("wrong read area")
	}
	if b.SaveLen() != 0 {
		t.Fatal("wrong save area")
	}

	b.Commit(10)

	if b.WriteLen() != 0 {
		t.Fatal("wrong write area")
	}
	if b.ReadLen() != 5 {
		t.Fatal("wrong read area")
	}
	if b.SaveLen() != 0 {
		t.Fatal("wrong save area")
	}

	var slots []Slot
	for i := 0; i < 10; i++ {
		slots = append(slots, b.Save(1))
	}

	if b.WriteLen() != 0 {
		t.Fatal("wrong write area")
	}
	if b.ReadLen() != 0 {
		t.Fatal("wrong read area")
	}
	if b.SaveLen() != 5 {
		t.Fatal("wrong save area")
	}

	var (
		offset   = 0
		expected = []byte("hello")
	)

	for i := 0; i < len(expected); i++ {
		if offset != i {
			t.Fatal("wrong offset")
		}
		slot := OffsetSlot(offset, slots[i])
		if !bytes.Equal(b.SavedSlot(slot), []byte{expected[i]}) {
			t.Fatal("wrong slot")
		}
		offset += b.Discard(slot)
	}
}

func TestByteBufferSaveAndDiscard3(t *testing.T) {
	b := NewByteBuffer()
	n, err := b.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatal("did not write 5")
	}
	b.Commit(5)
	var (
		slots    []Slot
		expected = []byte("hello")
	)
	for i := 0; i < 5; i++ {
		slots = append(slots, b.Save(1))
	}
	for i := len(slots) - 1; i >= 0; i-- {
		// no need to offset here
		if !bytes.Equal(b.SavedSlot(slots[i]), expected[i:i+1]) {
			t.Fatal("wrong slot")
		}
		b.Discard(slots[i])
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

func BenchmarkByteBufferClaim(b *testing.B) {
	buf := NewByteBuffer()
	for i := 0; i < b.N; i++ {
		buf.Claim(func(b []byte) int {
			return 1
		})
	}
	b.ReportAllocs()
}
