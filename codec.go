package sonic

// Codec defines a generic interface through which one can encode/decode
// a raw stream of bytes.
//
// Implementations are optionally able to track their state which enables
// writing both stateful and stateless parsers.
type Codec[Item any] interface {
	// Decode decodes the given stream into an `Item`.
	//
	// An implementation of Codec takes a byte stream that has already
	// been buffered in `src` and decodes the data into a stream of
	// `Item` objects.
	Decode(src *ByteBuffer) (Item, error)

	// Encode encodes the given item into the `dst` byte stream.
	Encode(item Item, dst *ByteBuffer) error
}

// BlockingCodecStream handles the decoding/encoding of bytes funneled through a
// provided blocking file descriptor.
type BlockingCodecStream[T any] struct {
	stream Stream
	codec  Codec[*T]
	src    *ByteBuffer
	dst    *ByteBuffer
}

func NewBlockingCodecStream[T any](stream Stream, codec Codec[*T], src, dst *ByteBuffer) (*BlockingCodecStream[T], error) {
	s := &BlockingCodecStream[T]{
		stream: stream,
		codec:  codec,
		src:    src,
	}
	return s, nil
}

func (s *BlockingCodecStream[T]) AsyncReadNext(cb func(error, *T)) {
	frame, err := s.codec.Decode(s.src)
	if err != nil {
		cb(err, nil)
	} else {
		if frame != nil {
			cb(nil, frame)
		} else {
			s.scheduleAsyncRead(cb)
		}
	}
}

func (s *BlockingCodecStream[T]) scheduleAsyncRead(cb func(error, *T)) {
	s.src.AsyncReadFrom(s.stream, func(err error, _ int) {
		if err != nil {
			cb(err, nil)
		} else {
			s.AsyncReadNext(cb)
		}
	})
}

func (s *BlockingCodecStream[T]) ReadNext() (*T, error) {
	for {
		frame, err := s.codec.Decode(s.src)
		if err != nil {
			return nil, err
		}

		if frame != nil {
			return frame, nil
		} else {
			_, err = s.src.ReadFrom(s.stream)
			if err != nil {
				return nil, err
			}
		}
	}
}

func (s *BlockingCodecStream[T]) WriteNext(frame *T) (n int, err error) {
	err = s.codec.Encode(frame, s.dst)
	if err == nil {
		var nn int64
		nn, err = s.dst.WriteTo(s.stream)
		n = int(nn)
	}
	return
}

func (s *BlockingCodecStream[T]) AsyncWriteNext(frame *T, cb AsyncCallback) {
	err := s.codec.Encode(frame, s.dst)
	if err == nil {
		s.dst.AsyncWriteTo(s.stream, cb)
	} else {
		cb(err, 0)
	}
}

func (s *BlockingCodecStream[T]) NextLayer() Stream {
	return s.stream
}
