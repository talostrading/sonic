package frame

import (
	"encoding/binary"
	"errors"

	"github.com/csdenboer/sonic"
	"github.com/csdenboer/sonic/sonicerrors"
)

var (
	_ sonic.Codec[[]byte, []byte] = &Codec{}

	ErrPayloadLengthOverflow = errors.New("payload length overflows")
)

const (
	HeaderLen        = 4                  // bytes
	MaxPayloadLength = 1024 * 1024 * 1024 // 1GB
)

type Codec struct {
	src *sonic.ByteBuffer

	decodeReset bool
	decodeBytes int
}

func NewCodec(src *sonic.ByteBuffer) *Codec {
	return &Codec{src: src}
}

func (c *Codec) Encode(frame []byte, dst *sonic.ByteBuffer) error {
	payloadLen := len(frame)

	if len(frame) > MaxPayloadLength {
		return ErrPayloadLengthOverflow
	}

	dst.Reserve(HeaderLen + payloadLen)

	dst.Claim(func(into []byte) int {
		binary.BigEndian.PutUint32(into[:HeaderLen], uint32(payloadLen))
		copy(into[HeaderLen:], frame)
		return HeaderLen + payloadLen
	})

	return nil
}

func (c *Codec) resetDecode() {
	if c.decodeReset {
		c.decodeReset = false
		c.src.Consume(c.decodeBytes)
		c.decodeBytes = 0
	}
}

func (c *Codec) Decode(src *sonic.ByteBuffer) ([]byte, error) {
	c.resetDecode()

	if err := src.PrepareRead(HeaderLen); err != nil {
		return nil, err
	}

	payloadLen := binary.BigEndian.Uint32(src.Data()[:HeaderLen])
	if payloadLen > MaxPayloadLength {
		return nil, ErrPayloadLengthOverflow
	}

	err := src.PrepareRead(HeaderLen + int(payloadLen))
	if err != nil {
		if err == sonicerrors.ErrNeedMore {
			src.Reserve(HeaderLen + int(payloadLen))
		}
		return nil, err
	}

	src.Consume(HeaderLen) // discard header; we are left with the payload

	c.decodeReset = true
	c.decodeBytes = int(payloadLen)

	return src.Data()[:payloadLen], nil
}
