package util

func PrependSlice[T any](x T, xs []T) []T {
	var emptyT T
	xs = append(xs, emptyT)
	copy(xs[1:], xs)
	xs[0] = x
	return xs
}

func ExtendSlice[T any](xs []T, need int) []T {
	xs = xs[:cap(xs)]
	if n := need - cap(xs); n > 0 {
		xs = append(xs, make([]T, n)...)
	}
	return xs[:need]
}

func CopySlice[T any](dst []T, src []T) []T {
	dst = ExtendSlice(dst, len(src))
	n := copy(dst, src)
	dst = dst[:n]
	return dst
}
