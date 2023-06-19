package util

import (
	"fmt"
	"io"
	"runtime/debug"
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

func TestFenwickTreeClear(t *testing.T) {
	tree := NewFenwickTree(5)

	tree.Clear(0)
	tree.Add(0, 20)
	tree.Add(4, 40)
	if tree.Sum() != 60 {
		t.Fatal("wrong sum")
	}
	tree.Clear(0)
	tree.Clear(4)
	if tree.Sum() != 0 {
		t.Fatal("wrong sum")
	}
}

func BenchmarkFenwickTreeAdd(b *testing.B) {
	tree := NewFenwickTree(1024 * 1024 * 16)
	for i := 0; i < b.N; i++ {
		tree.Add(i%tree.Size(), i)
	}
	fmt.Fprint(io.Discard, tree.Sum())
	b.ReportAllocs()
}

func trashedFenwickTree(n int) (*FenwickTree, *List[int]) {
	list := NewList[int]()

	tree := &FenwickTree{}
	for i := 0; i < n; i++ {
		tree.data = append(tree.data, 0)
		list.Add(i)
	}
	tree.data = tree.data[:n]

	return tree, list
}

func BenchmarkFenwickTreePrefixSum(b *testing.B) {
	debug.SetGCPercent(-1) // make sure trash is not picked up

	const N = 1024 * 10

	b.Run("fenwick_tree_prefix_sum", func(b *testing.B) {
		tree, _ := trashedFenwickTree(N)
		for i := 0; i < tree.Size(); i++ {
			tree.Add(i, i)
		}
		total := 0
		for i := 0; i < b.N; i++ {
			total += tree.SumUntil(i % tree.Size())
		}
		fmt.Fprint(io.Discard, total)
		b.ReportAllocs()
	})

	b.Run("array_linear_prefix_sum", func(b *testing.B) {
		var xs [N]int
		fn := func(index int) int {
			s := 0
			for i := 0; i <= index; i++ {
				s += xs[i]
			}
			return s
		}
		total := 0
		for i := 0; i < b.N; i++ {
			total += fn(i % N)
		}
		fmt.Fprint(io.Discard, total)
		b.ReportAllocs()
	})
}

func BenchmarkFenwickTreeRangeSum(b *testing.B) {
	debug.SetGCPercent(-1) // make sure trash is not picked up

	tree, _ := trashedFenwickTree(1024 * 10)

	for i := 0; i < tree.Size(); i++ {
		tree.Add(i, i)
	}
	total := 0
	for i := 0; i < b.N; i++ {
		total += tree.SumRange(
			i%tree.Size(),
			(i%tree.Size()+10)%tree.Size(),
		)
	}
	fmt.Fprint(io.Discard, total)
	b.ReportAllocs()
}
