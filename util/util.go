package util

func Prepend[T any](x T, xs []T) []T {
	var emptyT T
	xs = append(xs, emptyT)
	copy(xs[1:], xs)
	xs[0] = x
	return xs
}
