package util

import (
	"testing"
)

func TestList1(t *testing.T) {
	l := NewList[int]()
	for i := 0; i < 128; i++ {
		l.Add(0)
		if l.Size() != 1 {
			t.Fatal("wrong size")
		}
		if l.head == nil {
			t.Fatal("wrong head")
		}

		if !l.RemoveValue(0) {
			t.Fatal("wrong remove")
		}
		if l.Size() != 0 {
			t.Fatal("wrong size")
		}
		if l.head != nil {
			t.Fatal("wrong head")
		}
	}
}

func TestList2(t *testing.T) {
	l := NewList[int]()
	for i := 0; i < 128; i++ {
		l.Add(11)
		if l.Size() != 1 {
			t.Fatal("wrong size")
		}
		if l.head == nil {
			t.Fatal("wrong head")
		}

		if l.RemoveIndex(0) != 11 {
			t.Fatal("wrong remove")
		}
		if l.Size() != 0 {
			t.Fatal("wrong size")
		}
		if l.head != nil {
			t.Fatal("wrong head")
		}
	}
}

func TestList3(t *testing.T) {
	l := NewList[int]()
	for i := 0; i < 10; i++ {
		l.Add(i)
	}
	if l.Size() != 10 {
		t.Fatal("wrong Size")
	}
	for i := 0; i < 10; i++ {
		if l.At(i) != i {
			t.Fatal("wrong At")
		}
		if !l.Exists(i) {
			t.Fatal("wrong Exists")
		}
	}
	for i := 0; i < 10; i++ {
		if !l.RemoveValue(i) {
			t.Fatal("wrong Remove")
		}
		if l.Size() != 9-i {
			t.Fatal("wrong size")
		}
	}
}
