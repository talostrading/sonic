package sonic

import (
	"io"

	"github.com/talostrading/sonic/sonicerrors"
)

// ByteBuffer provides operations that make it easier to handle byte slices in networking code.
//
// Invariants:
//   - ri <= wi at all times
//   - wi = len(data) after each function call
type ByteBuffer struct {
	ri int // End index of the read area, always smaller or equal to wi.
	wi int // End index of the write area.

	oneByte [1]byte

	data []byte
}

// Interfaces which ByteBuffer implements.
var (
	_ io.Reader      = &ByteBuffer{}
	_ io.ByteReader  = &ByteBuffer{}
	_ io.ByteScanner = &ByteBuffer{}

	_ io.Writer     = &ByteBuffer{}
	_ io.ByteWriter = &ByteBuffer{}

	_ io.ReaderFrom   = &ByteBuffer{}
	_ AsyncReaderFrom = &ByteBuffer{}

	_ io.WriterTo   = &ByteBuffer{}
	_ AsyncWriterTo = &ByteBuffer{}
)

func NewByteBuffer() *ByteBuffer {
	b := &ByteBuffer{
		data: make([]byte, 0, 512),
	}
	return b
}

// Prepare prepares the buffer to hold `n` bytes.
func (b *ByteBuffer) Prepare(n int) {
	if need := n - cap(b.data); need > 0 {
		b.data = b.data[:cap(b.data)]
		b.data = append(b.data, make([]byte, need)...)
	}
	b.data = b.data[:b.wi]
}

// Commit moves `n` bytes from the write area to the read area.
func (b *ByteBuffer) Commit(n int) {
	if n < 0 {
		n = 0
	}

	b.ri += n
	if b.ri > b.wi {
		b.ri = b.wi
	}
}

// Data returns the bytes in the read area.
func (b *ByteBuffer) Data() []byte {
	return b.data[:b.ri]
}

// Len returns the length of the read area.
func (b *ByteBuffer) ReadLen() int {
	return len(b.data[:b.ri])
}

// UnreadLen returns the length of the write area.
func (b *ByteBuffer) WriteLen() int {
	return len(b.data[b.ri:b.wi])
}

// Len returns the length of the whole buffer.
func (b *ByteBuffer) Len() int {
	return len(b.data)
}

// Cap returns the capacity of the underlying byte slice.
func (b *ByteBuffer) Cap() int {
	return cap(b.data)
}

// Consume removes `n` bytes from the read area.
func (b *ByteBuffer) Consume(n int) {
	if n > b.ri {
		n = b.ri
	}

	if n != 0 {
		copy(b.data, b.data[n:b.wi])
	}

	b.wi -= n
	b.ri -= n

	b.data = b.data[:b.wi]
}

func (b *ByteBuffer) Reset() {
	b.data = b.data[:0]
	b.wi = 0
	b.ri = 0
}

// Reads reads and consumes the bytes from the read area into `dst`.
func (b *ByteBuffer) Read(dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}

	if b.ri == 0 {
		return 0, io.EOF
	}

	n := copy(dst, b.data[:b.ri])
	b.Consume(n)

	return n, nil
}

// ReadByte returns and consumes one byte from the read area.
func (b *ByteBuffer) ReadByte() (byte, error) {
	_, err := b.Read(b.oneByte[:])
	return b.oneByte[0], err
}

// ReadFrom reads from the provided Reader, placing the bytes in the write area.
func (b *ByteBuffer) ReadFrom(r io.Reader) (int64, error) {
	n, err := r.Read(b.data[b.wi:cap(b.data)])
	if err == nil {
		b.wi += n
		b.data = b.data[:b.wi]
	}
	return int64(n), err
}

func (b *ByteBuffer) UnreadByte() error {
	if can := b.wi - b.ri; can > 0 {
		b.wi -= 1
		b.data = b.data[:b.wi]
		return nil
	}
	return io.EOF
}

func (b *ByteBuffer) AsyncReadFrom(r AsyncReader, cb AsyncCallback) {
	r.AsyncRead(b.data[b.wi:cap(b.data)], func(err error, n int) {
		if err == nil {
			b.wi += n
			b.data = b.data[:b.wi]
		}
		cb(err, n)
	})
}

func (b *ByteBuffer) Write(bb []byte) (int, error) {
	b.data = append(b.data, bb...)
	n := len(bb)
	b.wi += n
	b.data = b.data[:b.wi]
	return n, nil
}

func (b *ByteBuffer) WriteByte(bb byte) error {
	b.data = append(b.data, bb)
	b.wi += 1
	b.data = b.data[:b.wi]
	return nil
}

func (b *ByteBuffer) WriteString(s string) (int, error) {
	b.data = append(b.data, s...)
	n := len(s)
	b.wi += n
	b.data = b.data[:b.wi]
	return n, nil
}

// WriteTo consumes and writes the bytes from the read area to the provided writer.
func (b *ByteBuffer) WriteTo(w io.Writer) (int64, error) {
	var (
		n            int
		err          error
		writtenBytes = 0
	)

	for {
		if writtenBytes >= b.ri {
			break
		}

		n, err = w.Write(b.data[writtenBytes:b.ri])
		writtenBytes += n
		if err != nil {
			break
		}
	}
	b.Consume(writtenBytes)
	return int64(writtenBytes), err
}

// WriteTo writes the bytes from the read area to the provided writer asynchronously.
func (b *ByteBuffer) AsyncWriteTo(w AsyncWriter, cb AsyncCallback) {
	w.AsyncWriteAll(b.data[:b.ri], func(err error, n int) {
		b.Consume(n)
		cb(err, n)
	})
}

// PrepareRead prepares n bytes to be read from the read area. If less than n bytes are
// available, io.EOF is returned.
func (b *ByteBuffer) PrepareRead(n int) (err error) {
	if need := n - b.ReadLen(); need > 0 {
		if b.WriteLen() >= need {
			b.Commit(need)
		} else {
			err = sonicerrors.ErrNeedMore
		}
	}
	return
}
