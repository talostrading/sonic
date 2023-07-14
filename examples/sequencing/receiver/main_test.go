package main

import (
	"fmt"
	"testing"

	"github.com/talostrading/sonic"
)

func TestSlowProcessor(t *testing.T) {
	p := NewSlowProcessor()
	b := sonic.NewByteBuffer()

	// in order
	p.Process(1, []byte("abcdef"), b)

	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 0 {
		t.Fatal("wrong")
	}
	fmt.Println(p.buffer)

	// out-of order
	p.Process(5, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 1 {
		t.Fatal("wrong")
	}
	fmt.Println(p.buffer)

	// out-of order
	p.Process(6, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 2 {
		t.Fatal("wrong")
	}
	fmt.Println(p.buffer)

	// out-of order
	p.Process(7, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 3 {
		t.Fatal("wrong")
	}
	fmt.Println(p.buffer)

	// out-of order
	p.Process(4, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 4 {
		t.Fatal("wrong")
	}
	fmt.Println(p.buffer)

	// duplicate out-of order
	p.Process(4, []byte("abcdef"), b)
	if p.expected != 2 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 4 {
		t.Fatal("wrong")
	}
	fmt.Println(p.buffer)

	// in order
	p.Process(2, []byte("abcdef"), b)
	if p.expected != 3 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 4 {
		t.Fatal("wrong")
	}
	fmt.Println(p.buffer)

	// in order, should also process all buffered
	p.Process(3, []byte("abcdef"), b)
	if p.expected != 8 {
		t.Fatal("wrong")
	}
	if len(p.buffer) != 0 {
		t.Fatal("wrong")
	}
	fmt.Println(p.buffer)
}

func TestFastProcessor(t *testing.T) {
	p := NewFastProcessor()
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
}
