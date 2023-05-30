package util

import (
	"testing"
)

func TestFenwickTree0(t *testing.T) {
	tree := NewFenwickTree(5)

	expected := []int{0, 0, 0, 0, 0}
	for i := 0; i < len(expected); i++ {
		if tree.At(i) != expected[i] {
			t.Fatal("At is wrong")
		}
	}
	if tree.Sum() != 0 {
		t.Fatal("wrong Sum")
	}

	tree.Add(0, 10)
	expected = []int{10, 0, 0, 0, 0}
	for i := 0; i < len(expected); i++ {
		if tree.At(i) != expected[i] {
			t.Fatal("At is wrong")
		}
	}
	if tree.Sum() != 10 {
		t.Fatal("wrong Sum")
	}

	tree.Add(4, 20)
	expected = []int{10, 0, 0, 0, 20}
	for i := 0; i < len(expected); i++ {
		if tree.At(i) != expected[i] {
			t.Fatal("At is wrong")
		}
	}
	if tree.Sum() != 30 {
		t.Fatal("wrong Sum")
	}
	if tree.SumFrom(0) != 30 {
		t.Fatal("wrong SumFrom")
	}
	if tree.SumUntil(4) != 30 {
		t.Fatal("wrong SumUntil")
	}
	if tree.SumRange(0, 4) != 30 {
		t.Fatal("wrong SumRange")
	}
	for i := 0; i < 4; i++ {
		if tree.SumUntil(i) != 10 {
			t.Fatal("wrong SumUntil")
		}
		if tree.SumRange(0, i) != 10 {
			t.Fatal("wrong SumRange")
		}
	}
	for i := 1; i < 5; i++ {
		if tree.SumFrom(i) != 20 {
			t.Fatal("wrong SumFrom")
		}
	}
	if tree.SumRange(1, 4) != 20 {
		t.Fatal("wrong SumRange")
	}

	tree.Add(2, 30)
	expected = []int{10, 0, 30, 0, 20}
	for i := 0; i < len(expected); i++ {
		if tree.At(i) != expected[i] {
			t.Fatal("At is wrong")
		}
	}
	if tree.Sum() != 60 {
		t.Fatal("wrong Sum")
	}
	if tree.SumRange(0, 4) != 60 {
		t.Fatal("wrong SumRange")
	}
	if tree.SumRange(1, 3) != 30 {
		t.Fatal("wrong SumRange")
	}
	if tree.SumFrom(3) != 20 {
		t.Fatal("wrong SumFrom")
	}
	if tree.SumFrom(4) != 20 {
		t.Fatal("wrong SumFrom")
	}
	if tree.SumUntil(2) != 40 {
		t.Fatal("wrong SumUntil")
	}

	tree.Add(0, -10)
	expected = []int{0, 0, 30, 0, 20}
	for i := 0; i < len(expected); i++ {
		if tree.At(i) != expected[i] {
			t.Fatal("At is wrong")
		}
	}

	tree.Add(2, -30)
	expected = []int{0, 0, 0, 0, 20}
	for i := 0; i < len(expected); i++ {
		if tree.At(i) != expected[i] {
			t.Fatal("At is wrong")
		}
	}
	if tree.Sum() != 20 {
		t.Fatal("wrong Sum")
	}
}
