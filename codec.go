package sonic

import (
	"errors"

	"github.com/talostrading/sonic/sonicerrors"
)

type Encoder[Item any] interface {
	// Encode encodes the given item into the `dst` byte stream.
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

// BlockingCodecStream handles the decoding/encoding of bytes funneled through a
// provided blocking file descriptor.
type BlockingCodecStream[Enc, Dec any] struct {
	stream Stream
	codec  Codec[Enc, Dec]
	src    *ByteBuffer
	dst    *ByteBuffer

	emptyEnc Enc
	emptyDec Dec
}

func NewBlockingCodecStream[Enc, Dec any](
	stream Stream,
	codec Codec[Enc, Dec],
	src, dst *ByteBuffer,
) (*BlockingCodecStream[Enc, Dec], error) {
	s := &BlockingCodecStream[Enc, Dec]{
		stream: stream,
		codec:  codec,
		src:    src,
		dst:    dst,
	}
	return s, nil
}

func (s *BlockingCodecStream[Enc, Dec]) AsyncReadNext(cb func(error, Dec)) {
	frame, err := s.codec.Decode(s.src)
	if errors.Is(err, sonicerrors.ErrNeedMore) {
		s.scheduleAsyncRead(cb)
	} else {
		cb(err, frame)
	}
}

func (s *BlockingCodecStream[Enc, Dec]) scheduleAsyncRead(cb func(error, Dec)) {
	s.src.AsyncReadFrom(s.stream, func(err error, _ int) {
		if err != nil {
			cb(err, s.emptyDec)
		} else {
			s.AsyncReadNext(cb)
		}
	})
}

func (s *BlockingCodecStream[Enc, Dec]) ReadNext() (Dec, error) {
	for {
		frame, err := s.codec.Decode(s.src)
		if err == nil {
			return frame, nil
		}

		if !errors.Is(err, sonicerrors.ErrNeedMore) {
			return s.emptyDec, err
		}

		_, err = s.src.ReadFrom(s.stream)
		if err != nil {
			return s.emptyDec, err
		}
	}
}

func (s *BlockingCodecStream[Enc, Dec]) WriteNext(item Enc) (n int, err error) {
	err = s.codec.Encode(item, s.dst)
	if err == nil {
		var nn int64
		nn, err = s.dst.WriteTo(s.stream)
		n = int(nn)
	}
	return
}

func (s *BlockingCodecStream[Enc, Dec]) AsyncWriteNext(item Enc, cb AsyncCallback) {
	err := s.codec.Encode(item, s.dst)
	if err == nil {
		s.dst.AsyncWriteTo(s.stream, cb)
	} else {
		cb(err, 0)
	}
}

func (s *BlockingCodecStream[Enc, Dec]) NextLayer() Stream {
	return s.stream
}
