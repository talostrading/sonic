package util

func ExtendByteSlice(b []byte, need int) []byte {
	b = b[:cap(b)]
	if n := need - cap(b); n > 0 {
		b = append(b, make([]byte, n)...)
	}
	return b[:need]
}
