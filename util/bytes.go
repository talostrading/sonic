package util

func ExtendBytes(b []byte, need int) []byte {
	b = b[:cap(b)]
	if n := need - cap(b); n > 0 {
		b = append(b, make([]byte, n)...)
	}
	return b[:need]
}

func CopyBytes(dst []byte, src []byte) []byte {
	dst = ExtendBytes(dst, len(src))
	n := copy(dst, src)
	dst = dst[:n]
	return dst
}
