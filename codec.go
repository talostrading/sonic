package sonic

import (
	"errors"
	"fmt"

	"github.com/talostrading/sonic/internal"
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

type CodecConn[Enc, Dec any] interface {
	AsyncReadNext(func(error, Dec))
	ReadNext() (Dec, error)

	AsyncWriteNext(Enc, AsyncCallback)
	WriteNext(Enc) (int, error)

	NextLayer() Stream

	Close() error
}

var (
	_ CodecConn[any, any] = &BlockingCodecConn[any, any]{}
	_ CodecConn[any, any] = &NonblockingCodecConn[any, any]{}
)

// BlockingCodecConn handles the decoding/encoding of bytes funneled through a
// provided blocking file descriptor.
type BlockingCodecConn[Enc, Dec any] struct {
	stream Stream
	codec  Codec[Enc, Dec]
	src    *ByteBuffer
	dst    *ByteBuffer

	emptyEnc Enc
	emptyDec Dec
}

func NewBlockingCodecConn[Enc, Dec any](
	stream Stream,
	codec Codec[Enc, Dec],
	src, dst *ByteBuffer,
) (*BlockingCodecConn[Enc, Dec], error) {
	// Works on both blocking and nonblocking fds.

	c := &BlockingCodecConn[Enc, Dec]{
		stream: stream,
		codec:  codec,
		src:    src,
		dst:    dst,
	}
	return c, nil
}

func (c *BlockingCodecConn[Enc, Dec]) AsyncReadNext(cb func(error, Dec)) {
	item, err := c.codec.Decode(c.src)
	if errors.Is(err, sonicerrors.ErrNeedMore) {
		c.scheduleAsyncRead(cb)
	} else {
		cb(err, item)
	}
}

func (c *BlockingCodecConn[Enc, Dec]) scheduleAsyncRead(cb func(error, Dec)) {
	c.src.AsyncReadFrom(c.stream, func(err error, _ int) {
		if err != nil {
			cb(err, c.emptyDec)
		} else {
			c.AsyncReadNext(cb)
		}
	})
}

func (c *BlockingCodecConn[Enc, Dec]) ReadNext() (Dec, error) {
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

func (c *BlockingCodecConn[Enc, Dec]) WriteNext(item Enc) (n int, err error) {
	err = c.codec.Encode(item, c.dst)
	if err == nil {
		var nn int64
		nn, err = c.dst.WriteTo(c.stream)
		n = int(nn)
	}
	return
}

func (c *BlockingCodecConn[Enc, Dec]) AsyncWriteNext(item Enc, cb AsyncCallback) {
	err := c.codec.Encode(item, c.dst)
	if err == nil {
		c.dst.AsyncWriteTo(c.stream, cb)
	} else {
		cb(err, 0)
	}
}

func (c *BlockingCodecConn[Enc, Dec]) NextLayer() Stream {
	return c.stream
}

func (c *BlockingCodecConn[Enc, Dec]) Close() error {
	return c.stream.Close()
}

type NonblockingCodecConn[Enc, Dec any] struct {
	stream Stream
	codec  Codec[Enc, Dec]
	src    *ByteBuffer
	dst    *ByteBuffer

	dispatched int

	emptyEnc Enc
	emptyDec Dec
}

func NewNonblockingCodecConn[Enc, Dec any](
	stream Stream,
	codec Codec[Enc, Dec],
	src, dst *ByteBuffer,
) (*NonblockingCodecConn[Enc, Dec], error) {
	nonblocking, err := internal.IsNonblocking(stream.RawFd())
	if err != nil {
		return nil, err
	}
	if !nonblocking {
		return nil, fmt.Errorf("the provided Stream is blocking")
	}

	c := &NonblockingCodecConn[Enc, Dec]{
		stream: stream,
		codec:  codec,
		src:    src,
		dst:    dst,
	}
	return c, nil
}

func (c *NonblockingCodecConn[Enc, Dec]) AsyncReadNext(cb func(error, Dec)) {
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

func (c *NonblockingCodecConn[Enc, Dec]) ReadNext() (Dec, error) {
	for {
		item, err := c.codec.Decode(c.src)
		if err == nil {
			return item, nil
		}

		if err != sonicerrors.ErrNeedMore {
			return c.emptyDec, err
		}

		_, err = c.src.ReadFrom(c.stream)
		if err != nil {
			return c.emptyDec, err
		}
	}
}

func (c *NonblockingCodecConn[Enc, Dec]) AsyncWriteNext(item Enc, cb AsyncCallback) {
	if err := c.codec.Encode(item, c.dst); err != nil {
		cb(err, 0)
		return
	}

	// write everything into `dst`
	c.dst.AsyncWriteTo(c.stream, cb)
}

func (c *NonblockingCodecConn[Enc, Dec]) WriteNext(item Enc) (n int, err error) {
	err = c.codec.Encode(item, c.dst)
	if err == nil {
		var nn int64
		nn, err = c.dst.WriteTo(c.stream)
		n = int(nn)
	}
	return
}

func (c *NonblockingCodecConn[Enc, Dec]) NextLayer() Stream {
	return c.stream
}

func (c *NonblockingCodecConn[Enc, Dec]) Close() error {
	return c.stream.Close()
}
