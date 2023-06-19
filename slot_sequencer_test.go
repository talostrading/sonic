package sonic

import (
	"log"
	"math/rand"
	"testing"
	"time"
)

func TestSlotSequencer1(t *testing.T) {
	b := NewByteBuffer()
	s := NewSlotSequencer(10, 1024)

	letters := make(map[int][]byte)

	push := func(seq int, letter byte, n int) {
		var what []byte
		for i := 0; i < n; i++ {
			what = append(what, letter)
		}
		letters[seq] = what

		b.Write(what)
		b.Commit(n)
		ok, err := s.Push(seq, b.Save(n))
		if !ok || err != nil {
			t.Fatalf("not pushed ok=%v err=%v", ok, err)
		}
	}

	pop := func(seq int) {
		slot, ok := s.Pop(seq)
		if !ok {
			t.Fatal("not popped")
		}
		expected := string(letters[seq])
		given := string(b.SavedSlot(slot))
		if expected != given {
			t.Fatalf("wrong slot expected=%s given=%s", expected, given)
		}
		b.Discard(slot)
	}

	push(2, 'b', 4)
	push(1, 'a', 2)
	push(4, 'd', 8)
	push(3, 'c', 6)
	push(5, 'e', 10)

	if s.Bytes() != 30 {
		t.Fatal("wrong number of bytes")
	}

	for i := 5; i >= 1; i-- {
		pop(i)
	}
	if s.offsetter.tree.Sum() != 0 {
		t.Fatal("offsetter should have been cleared")
	}
	if s.Bytes() != 0 {
		t.Fatal("slot manager should have 0 bytes")
	}
}

func TestSlotSequencerRandom(t *testing.T) {
	b := NewByteBuffer()
	s := NewSlotSequencer(4096, 1024*1024)

	letters := make(map[int][]byte)

	push := func(seq int, letter byte, n int) {
		var what []byte
		for i := 0; i < n; i++ {
			what = append(what, letter)
		}
		letters[seq] = what

		b.Write(what)
		b.Commit(n)
		ok, err := s.Push(seq, b.Save(n))
		if !ok || err != nil {
			t.Fatalf("not pushed ok=%v err=%v", ok, err)
		}
	}

	pop := func(seq int) {
		slot, ok := s.Pop(seq)
		if !ok {
			t.Fatal("not popped")
		}
		expected := string(letters[seq])
		given := string(b.SavedSlot(slot))
		if expected != given {
			t.Fatalf("wrong slot expected=%s given=%s", expected, given)
		}
		b.Discard(slot)
	}

	alphabet := []byte("abcdefghijklmnopqrstuvwxyz")

	rand := rand.New(rand.NewSource(time.Now().UnixNano()))

	iterations := 0
	start := time.Now()
	for time.Since(start).Seconds() < 30 {
		var sequences []int
		for seq := 1; seq <= 1024; seq++ {
			sequences = append(sequences, seq)
		}

		for len(sequences) > 0 {
			var toPop []int

			nPop := rand.Int()%len(sequences) + 1
			for i := 0; i < nPop; i++ {
				ix := rand.Int() % len(sequences)
				seq := sequences[ix]
				sequences = append(sequences[:ix], sequences[ix+1:]...)

				toPop = append(toPop, seq)

				push(seq, alphabet[rand.Int()%len(alphabet)], rand.Int()%100+1)
			}

			for len(toPop) > 0 {
				ix := rand.Int() % len(toPop)
				pop(toPop[ix])
				toPop = append(toPop[:ix], toPop[ix+1:]...)
			}
		}

		if s.offsetter.tree.Sum() != 0 {
			t.Fatal("offsetter should have been cleared")
		}

		if s.Bytes() != 0 {
			t.Fatal("slot manager should have 0 bytes")
		}

		iterations++

	}

	log.Printf("slot manager random test iterations=%d", iterations)
}
