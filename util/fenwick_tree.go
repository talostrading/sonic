package util

// TODO
// - point query
// - range query
// - range update

type FenwickTree struct {
	data []int
}

func NewFenwickTree(n int) *FenwickTree {
	t := &FenwickTree{}
	t.data = ExtendSlice(t.data, n)
	return t
}

func NewFenwickTreeFrom(xs []int) *FenwickTree {
	t := NewFenwickTree(len(xs))

	for i, x := range xs {
		t.data[i] += x
		after := i | (i + 1)
		if after < len(xs) {
			t.data[after] += t.data[i]
		}
	}

	return t
}

func (t *FenwickTree) Add(index, delta int) {
	for index < len(t.data) {
		t.data[index] += delta
		index = index | (index + 1)
	}
}

func (t *FenwickTree) SumUntil(index int) (sum int) {
	for index >= 0 {
		sum += t.data[index]
		index = index&(index+1) - 1
	}
	return sum
}

func (t *FenwickTree) SumFrom(index int) int {
	return t.Sum() - t.SumUntil(index-1)
}

func (t *FenwickTree) Sum() int {
	return t.SumUntil(len(t.data) - 1)
}

func (t *FenwickTree) At(index int) int {
	// TODO there's probably a better way
	return t.SumRange(index, index)
}

func (t *FenwickTree) Clear(index int) int {
	v := t.At(index)
	t.Add(index, -v)
	return v
}

func (t *FenwickTree) Reset() {
	data := t.data
	for i := range data {
		data[i] = 0
	}
}

func (t *FenwickTree) SumRange(left, right int) int {
	return t.SumUntil(right) - t.SumUntil(left-1)
}

func (t *FenwickTree) Size() int {
	return len(t.data)
}
