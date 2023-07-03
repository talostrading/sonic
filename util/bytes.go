package util

import "fmt"

func ByteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

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
