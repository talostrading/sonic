package sonic

import (
	"errors"
	"github.com/talostrading/sonic/sonicerrors"
)

type Encoder[Item any] interface {
	// Encode encodes the given item into the `dst` byte stream.
	//
	// Implementations should:
	// - Commit the bytes into the read area of `dst`.
	// - ensure `dst` is big enough to hold the serialized item by
	//   calling dst.Reserve(...)
	Encode(item Item, dst *ByteBuffer) error
}

type Decoder[Item any] interface {
	// Decode decodes the given stream into an `Item`.
	//
	// An implementation of Codec takes a byte stream that has already
	// been buffered in `src` and decodes the data into a stream of
	// `Item` objects.
	//
	// Implementations should return an empty Item and ErrNeedMore if
	// there are not enough bytes to decode into an Item.
	Decode(src *ByteBuffer) (Item, error)
}

// Codec defines a generic interface through which one can encode/decode
// a raw stream of bytes.
//
// Implementations are optionally able to track their state which enables
// writing both stateful and stateless parsers.
type Codec[Enc, Dec any] interface {
	Encoder[Enc]
	Decoder[Dec]
}

// CodecConn handles the decoding/encoding of bytes funneled through a
// provided blocking file descriptor.
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
	// Works on both blocking and nonblocking fds.

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
