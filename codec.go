package sonic

import (
	"errors"
	"github.com/talostrading/sonic/sonicerrors"
	"github.com/talostrading/sonic/sonicopts"
)

func NewCodecConn[Enc, Dec any](
	ioc *IO,
	conn Conn,
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
		return newBlockingCodecConn(ioc, conn, codec, src, dst)
	}
}

// TODO NonblockingCodecConn

// BlockingCodecConn handles the decoding/encoding of bytes funneled through a
// provided blocking nonblockingFd descriptor.
type BlockingCodecConn[Enc, Dec any] struct {
	ioc      *IO
	conn     Conn
	codec    Codec[Enc, Dec]
	src, dst *ByteBuffer

	emptyEnc Enc
	emptyDec Dec
}

var _ CodecConn[any, any] = &BlockingCodecConn[interface{}, interface{}]{}

func newBlockingCodecConn[Enc, Dec any](
	ioc *IO,
	conn Conn,
	codec Codec[Enc, Dec],
	src, dst *ByteBuffer,
) (*BlockingCodecConn[Enc, Dec], error) {
	s := &BlockingCodecConn[Enc, Dec]{
		ioc:   ioc,
		conn:  conn,
		codec: codec,
		src:   src,
		dst:   dst,
	}
	return s, nil
}

func (s *BlockingCodecConn[Enc, Dec]) AsyncReadNext(cb func(error, Dec)) {
	item, err := s.codec.Decode(s.src)
	if errors.Is(err, sonicerrors.ErrNeedMore) {
		s.scheduleAsyncRead(cb)
	} else {
		cb(err, item)
	}
}

func (s *BlockingCodecConn[Enc, Dec]) scheduleAsyncRead(cb func(error, Dec)) {
	s.src.AsyncReadFrom(s.conn, func(err error, _ int) {
		if err != nil {
			cb(err, s.emptyDec)
		} else {
			s.AsyncReadNext(cb)
		}
	})
}

func (s *BlockingCodecConn[Enc, Dec]) ReadNext() (Dec, error) {
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

func (s *BlockingCodecConn[Enc, Dec]) WriteNext(item Enc) (err error) {
	err = s.codec.Encode(item, s.dst)
	if err == nil {
		_, err = s.dst.WriteTo(s.conn)
	}
	return
}

func (s *BlockingCodecConn[Enc, Dec]) AsyncWriteNext(item Enc, cb func(error)) {
	err := s.codec.Encode(item, s.dst)
	if err == nil {
		s.dst.AsyncWriteTo(s.conn, func(err error, _ int) {
			cb(err)
		})
	} else {
		cb(err)
	}
}

func (s *BlockingCodecConn[Enc, Dec]) NextLayer() Conn {
	return s.conn
}

func (s *BlockingCodecConn[Enc, Dec]) Close() error {
	return s.conn.Close()
}

func (s *BlockingCodecConn[Enc, Dec]) Closed() bool {
	return s.conn.Closed()
}
