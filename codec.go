package sonic

import (
	"errors"
	"github.com/talostrading/sonic/sonicerrors"
)

type Encoder[Item any] interface {
	// Encode the given `Item` into the given buffer.
	//
	// Implementations should:
	// - Commit() the bytes into the given buffer if the encoding is successful.
	// - Ensure the given buffer is big enough to hold the serialized `Item`s by calling `Reserve(...)`.
	Encode(item Item, dst *ByteBuffer) error
}

type Decoder[Item any] interface {
	// Decode the next `Item`, if any, from the provided buffer. If there are not enough bytes to decode an `Item`,
	// implementations should return an empty `Item` along with `ErrNeedMore`. `CodecConn` will then know to read more
	// bytes before calling `Decode(...)` again.
	Decode(src *ByteBuffer) (Item, error)
}

// Codec groups together and Encoder and a Decoder for a CodecConn.
type Codec[Enc, Dec any] interface {
	Encoder[Enc]
	Decoder[Dec]
}

// CodecConn reads/writes `Item`s through the provided `Codec`. For an example, see `codec/frame.go`.
type CodecConn[Enc, Dec any] struct {
	stream Stream
	codec  Codec[Enc, Dec]
	src    *ByteBuffer
	dst    *ByteBuffer

	emptyEnc Enc
	emptyDec Dec
}

func NewCodecConn[Enc, Dec any](
	stream Stream,
	codec Codec[Enc, Dec],
	src, dst *ByteBuffer,
) (*CodecConn[Enc, Dec], error) {
	c := &CodecConn[Enc, Dec]{
		stream: stream,
		codec:  codec,
		src:    src,
		dst:    dst,
	}
	return c, nil
}

func (c *CodecConn[Enc, Dec]) AsyncReadNext(cb func(error, Dec)) {
	item, err := c.codec.Decode(c.src)
	if errors.Is(err, sonicerrors.ErrNeedMore) {
		c.src.AsyncReadFrom(c.stream, func(err error, _ int) {
			if err != nil {
				cb(err, c.emptyDec)
			} else {
				c.AsyncReadNext(cb)
			}
		})
	} else {
		cb(err, item)
	}
}

func (c *CodecConn[Enc, Dec]) ReadNext() (Dec, error) {
	for {
		item, err := c.codec.Decode(c.src)
		if err == nil {
			return item, nil
		}

		if !errors.Is(err, sonicerrors.ErrNeedMore) {
			return c.emptyDec, err
		}

		_, err = c.src.ReadFrom(c.stream)
		if err != nil {
			return c.emptyDec, err
		}
	}
}

func (c *CodecConn[Enc, Dec]) WriteNext(item Enc) (n int, err error) {
	err = c.codec.Encode(item, c.dst)
	if err == nil {
		var nn int64
		nn, err = c.dst.WriteTo(c.stream)
		n = int(nn)
	}
	return
}

func (c *CodecConn[Enc, Dec]) AsyncWriteNext(item Enc, cb AsyncCallback) {
	err := c.codec.Encode(item, c.dst)
	if err == nil {
		c.dst.AsyncWriteTo(c.stream, cb)
	} else {
		cb(err, 0)
	}
}

func (c *CodecConn[Enc, Dec]) NextLayer() Stream {
	return c.stream
}

func (c *CodecConn[Enc, Dec]) Close() error {
	return c.stream.Close()
}
