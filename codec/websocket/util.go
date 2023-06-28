package websocket

/*
gosec G505 G401:
The WebSocket protocol mandates the use of sha1 hashes in the
opening handshake initiated by the client. Sha1 is used purely
as a hashing function. This hash not used to provide any security.
It is simply used as a verification step by the protocol to ensure
that the server speaks the WebSocket protocol.
This verifiction is needed as the handshake is done over HTTP -
without it any http server might accept the websocket handshake, at
which point the protocol will be violated on subsequent read/writes,
when the client cannot parse what the server sends.
*/

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
