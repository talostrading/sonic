package websocket

import (
	"crypto/rand"
	"encoding/base64"
	"github.com/talostrading/sonic/util"
	"hash"
)

func Mask(mask, b []byte) {
	for i := range b {
		b[i] ^= mask[i&3]
	}
}

func GenMask(b []byte) {
	rand.Read(b)
}

func MakeClientRequestKey(b []byte) string {
	b = util.ExtendSlice(b, 16)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func MakeServerResponseKey(hasher hash.Hash, reqKey []byte) string {
	var resKey []byte
	resKey = append(resKey, reqKey...)
	resKey = append(resKey, GUID...)

	hasher.Reset()
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
