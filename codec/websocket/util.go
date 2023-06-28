package websocket

import (
	"crypto/rand"
	"crypto/sha1" //#nosec G505
	"encoding/base64"
)

func Mask(mask, b []byte) {
	for i := range b {
		b[i] ^= mask[i&3]
	}
}

func GenMask(b []byte) {
	_, _ = rand.Read(b)
}

func MakeRequestKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func MakeResponseKey(reqKey []byte) string {
	var resKey []byte
	resKey = append(resKey, reqKey...)
	resKey = append(resKey, GUID...)

	hasher := sha1.New() //#nosec G401
	hasher.Write(resKey)
	return base64.StdEncoding.EncodeToString(hasher.Sum(nil))
}

func EncodeCloseFramePayload(cc CloseCode, reason string) []byte {
	b := EncodeCloseCode(cc)
	b = append(b, []byte(reason)...)
	return b
}

func DecodeCloseFramePayload(b []byte) (cc CloseCode, reason string) {
	cc = DecodeCloseCode(b[:2])
	reason = string(b[2:])
	return
}
