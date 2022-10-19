package sonic

import (
	"errors"
	"github.com/talostrading/sonic/sonicopts"

	"github.com/talostrading/sonic/sonicerrors"
)

type Encoder[Item any] interface {
	// Encode encodes the given `Item` into the `dst` buffer.
	Encode(item Item, dst *ByteBuffer) error
}

type Decoder[Item any] interface {
	// Decode decodes the read area of the buffer into an `Item`.
	//
	// An implementation of Codec takes a byte stream that has already
	// been buffered in `src` and decodes the data into `Item`s.
	//
	// Implementations should return an empty Item and ErrNeedMore if
	// there are not enough bytes to decode into an Item.
	Decode(src *ByteBuffer) (Item, error)
}

// Codec defines a generic interface through which one can encode/decode raw bytes.
//
// Implementations are optionally able to track their state which enables
// writing both stateful and stateless parsers.
type Codec[Enc, Dec any] interface {
	Encoder[Enc]
	Decoder[Dec]
}

// CodecConn defines a generic interface through which a stream
// of bytes from Conn can be encoded/decoded with a Codec.
type CodecConn[Enc, Dec any] interface {
	ReadNext() (Dec, error)
	AsyncReadNext(func(error, Dec))

	WriteNext(Enc) error
	AsyncWriteNext(Enc, func(error))

	NextLayer() FileDescriptor // TODO this should be Conn
}

func NewCodecConn[Enc, Dec any](
	ioc *IO,
	conn FileDescriptor,
	codec Codec[Enc, Dec],
	src, dst *ByteBuffer,
	opts ...sonicopts.Option,
) (codecConn CodecConn[Enc, Dec], err error) {
	nonblocking := false
	for _, opt := range opts {
		switch opt.Type() {
		case sonicopts.TypeNonblocking:
			nonblocking = opt.Value().(bool)
		}
	}

	if nonblocking {
		panic("not implemented")
	} else {
		return newBlockingCodecStream(ioc, conn, codec, src, dst)
	}
}

// BlockingCodecStream handles the decoding/encoding of bytes funneled through a
// provided blocking file descriptor.
type BlockingCodecStream[Enc, Dec any] struct {
	ioc      *IO
	conn     FileDescriptor
	codec    Codec[Enc, Dec]
	src, dst *ByteBuffer

	emptyEnc Enc
	emptyDec Dec
}

var _ CodecConn[any, any] = &BlockingCodecStream[interface{}, interface{}]{}

func newBlockingCodecStream[Enc, Dec any](
	ioc *IO,
	conn FileDescriptor,
	codec Codec[Enc, Dec],
	src, dst *ByteBuffer,
) (*BlockingCodecStream[Enc, Dec], error) {
	s := &BlockingCodecStream[Enc, Dec]{
		ioc:   ioc,
		conn:  conn,
		codec: codec,
		src:   src,
		dst:   dst,
	}
	return s, nil
}

func (s *BlockingCodecStream[Enc, Dec]) AsyncReadNext(cb func(error, Dec)) {
	item, err := s.codec.Decode(s.src)
	if errors.Is(err, sonicerrors.ErrNeedMore) {
		s.scheduleAsyncRead(cb)
	} else {
		cb(err, item)
	}
}

func (s *BlockingCodecStream[Enc, Dec]) scheduleAsyncRead(cb func(error, Dec)) {
	s.src.AsyncReadFrom(s.conn, func(err error, _ int) {
		if err != nil {
			cb(err, s.emptyDec)
		} else {
			s.AsyncReadNext(cb)
		}
	})
}

func (s *BlockingCodecStream[Enc, Dec]) ReadNext() (Dec, error) {
	for {
		item, err := s.codec.Decode(s.src)
		if err == nil {
			return item, nil
		}

		if !errors.Is(err, sonicerrors.ErrNeedMore) {
			return s.emptyDec, err
		}

		_, err = s.src.ReadFrom(s.conn)
		if err != nil {
			return s.emptyDec, err
		}
	}
}

func (s *BlockingCodecStream[Enc, Dec]) WriteNext(item Enc) (err error) {
	err = s.codec.Encode(item, s.dst)
	if err == nil {
		_, err = s.dst.WriteTo(s.conn)
	}
	return
}

func (s *BlockingCodecStream[Enc, Dec]) AsyncWriteNext(item Enc, cb func(error)) {
	err := s.codec.Encode(item, s.dst)
	if err == nil {
		s.dst.AsyncWriteTo(s.conn, func(err error, _ int) {
			cb(err)
		})
	} else {
		cb(err)
	}
}

func (s *BlockingCodecStream[Enc, Dec]) NextLayer() FileDescriptor {
	return s.conn
}
