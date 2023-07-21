package main

import (
	"testing"

	"github.com/talostrading/sonic"
)

func TestAllocProcessor(t *testing.T) {
	p := NewAllocProcessor()

	b := sonic.NewByteBuffer()

	// in order
	p.Process(1, []byte("abcdef"), b)

	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 0 {
		t.Fatal("wrong")
	}

	// out-of order
	p.Process(5, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 1 {
		t.Fatal("wrong")
	}

	// out-of order
	p.Process(6, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 2 {
		t.Fatal("wrong")
	}

	// out-of order
	p.Process(7, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 3 {
		t.Fatal("wrong")
	}

	// out-of order
	p.Process(4, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 4 {
		t.Fatal("wrong")
	}

	// duplicate out-of order
	p.Process(4, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 4 {
		t.Fatal("wrong")
	}

	// in order
	p.Process(2, []byte("abcdef"), b)
	if p.expected != 3 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 4 {
		t.Fatal("wrong")
	}

	// in order, should also process all buffered
	p.Process(3, []byte("abcdef"), b)
	if p.expected != 8 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 0 {
		t.Fatal("wrong")
	}

	// now send some older ones, should be ignored
	p.Process(2, []byte("abc"), b)
	if p.expected != 8 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 0 {
		t.Fatal("wrong")
	}

	p.Process(3, []byte("abc"), b)
	if p.expected != 8 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 0 {
		t.Fatal("wrong")
	}
}

func TestPoolAllocProcessor(t *testing.T) {
	p := NewPoolAllocProcessor()

	b := sonic.NewByteBuffer()
	// in order
	p.Process(1, []byte("abcdef"), b)

	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 0 {
		t.Fatal("wrong")
	}

	// out-of order
	p.Process(5, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 1 {
		t.Fatal("wrong")
	}

	// out-of order
	p.Process(6, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 2 {
		t.Fatal("wrong")
	}

	// out-of order
	p.Process(7, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 3 {
		t.Fatal("wrong")
	}

	// out-of order
	p.Process(4, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 4 {
		t.Fatal("wrong")
	}

	// duplicate out-of order
	p.Process(4, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 4 {
		t.Fatal("wrong")
	}

	// in order
	p.Process(2, []byte("abcdef"), b)
	if p.expected != 3 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 4 {
		t.Fatal("wrong")
	}

	// in order, should also process all buffered
	p.Process(3, []byte("abcdef"), b)
	if p.expected != 8 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 0 {
		t.Fatal("wrong")
	}

	// now send some older ones, should be ignored
	p.Process(2, []byte("abc"), b)
	if p.expected != 8 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 0 {
		t.Fatal("wrong")
	}

	p.Process(3, []byte("abc"), b)
	if p.expected != 8 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 0 {
		t.Fatal("wrong")
	}
}

func TestNoAllocProcessor(t *testing.T) {
	p := NewNoAllocProcessor()
	b := sonic.NewByteBuffer()

	// first
	p.Process(1, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if p.sequencer.Size() != 0 {
		t.Fatal("wrong")
	}

	// out of order
	p.Process(4, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if p.sequencer.Size() != 1 {
		t.Fatal("wrong")
	}

	// this should be offset with the fenwick tree cause it comes after 5
	p.Process(6, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if p.sequencer.Size() != 2 {
		t.Fatal("wrong")
	}

	p.Process(5, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if p.sequencer.Size() != 3 {
		t.Fatal("wrong")
	}

	p.Process(5, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if p.sequencer.Size() != 3 {
		t.Fatal("wrong")
	}

	// in order
	p.Process(2, []byte("abc"), b)
	if p.expected != 3 {
		t.Fatal("wrong")
	}
	if p.sequencer.Size() != 3 {
		t.Fatal("wrong")
	}

	p.Process(3, []byte("abc"), b)
	if p.expected != 7 {
		t.Fatal("wrong")
	}
	if p.sequencer.Size() != 0 {
		t.Fatal("wrong")
	}

	// now send some older ones, should be ignored
	p.Process(2, []byte("abc"), b)
	if p.expected != 7 {
		t.Fatal("wrong")
	}
	if p.sequencer.Size() != 0 {
		t.Fatal("wrong")
	}

	p.Process(3, []byte("abc"), b)
	if p.expected != 7 {
		t.Fatal("wrong")
	}
	if p.sequencer.Size() != 0 {
		t.Fatal("wrong")
	}
}

func BenchmarkWalkBuffer(b *testing.B) {
	buf := sonic.NewByteBuffer()
	buf.Reserve(1024 * 256)
	buf.Warm()

	sequencer := sonic.NewSlotSequencer(1024, 1024*256)

	walk := func(seq int) int {
		for {
			slot, ok := sequencer.Pop(seq)
			if !ok {
				break
			}

			seq++

			_ = buf.SavedSlot(slot)
			buf.Discard(slot)
		}
		return seq
	}

	seq := 1

	var slice []byte
	for i := 0; i < 128; i++ {
		slice = append(slice, 1)
	}

	for i := 0; i < b.N; i++ {
		buf.Write(slice)
		buf.Commit(len(slice))
		slot := buf.Save(len(slice))
		sequencer.Push(seq, slot)

		seq = walk(seq)
	}

	b.ReportAllocs()
}
