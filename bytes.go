package sonic

import (
	"io"
)

// Interfaces to which BytesBuffer adheres to.
var (
	_ io.Reader      = &BytesBuffer{}
	_ io.ByteReader  = &BytesBuffer{}
	_ io.ByteScanner = &BytesBuffer{}

	_ io.Writer     = &BytesBuffer{}
	_ io.ByteWriter = &BytesBuffer{}

	_ io.ReaderFrom   = &BytesBuffer{}
	_ AsyncReaderFrom = &BytesBuffer{}

	_ io.WriterTo   = &BytesBuffer{}
	_ AsyncWriterTo = &BytesBuffer{}
)

// BytesBuffer provides operations that make it easier to handle byte slices in networking code.
type BytesBuffer struct {
	ri int // End index of the read area, always smaller or equal to wi.
	wi int // End index of the write area.

	oneByte [1]byte

	data []byte
}

func NewBytesBuffer() *BytesBuffer {
	b := &BytesBuffer{
		data: make([]byte, 512),
	}
	return b
}

// Prepare prepares the buffer to read `n` bytes.
//
// The `n` bytes will be read into the write area.
func (b *BytesBuffer) Prepare(n int) {
	if need := n - cap(b.data); need > 0 {
		b.data = append(b.data, make([]byte, need)...)
	}
}

// Commit moves `n` bytes from the write area to the read area.
func (b *BytesBuffer) Commit(n int) {
	if n < 0 {
		n = 0
	}

	b.ri += n
	if b.ri > b.wi {
		b.ri = b.wi
	}
}

// Data returns the bytes in the read area.
func (b *BytesBuffer) Data() []byte {
	return b.data[:b.ri]
}

// Len returns the length of the read area.
func (b *BytesBuffer) ReadLen() int {
	return len(b.data[:b.ri])
}

// UnreadLen returns the length of the write area.
func (b *BytesBuffer) WriteLen() int {
	return len(b.data[b.ri:b.wi])
}

// Cap returns the capacity of the underlying byte slice.
func (b *BytesBuffer) Cap() int {
	return cap(b.data)
}

// Consume removes `n` bytes from the read area.
func (b *BytesBuffer) Consume(n int) {
	if n > b.ri {
		n = b.ri
	}

	if n != 0 {
		copy(b.data, b.data[n:b.wi])
	}

	b.wi -= n
	b.ri -= n
}

func (b *BytesBuffer) Reset() {
	b.data = b.data[:0]
	b.wi = 0
	b.ri = 0
}

// Reads reads into `dst` by consuming the bytes in the read area.
func (b *BytesBuffer) Read(dst []byte) (int, error) {
	if b.ri == b.wi {
		return 0, nil
	}

	n := copy(dst, b.data[b.ri:b.wi])
	b.Consume(n)

	return n, nil
}

// ReadByte consumes and returns one byte from the read area.
func (b *BytesBuffer) ReadByte() (byte, error) {
	n, err := b.Read(b.oneByte[:])
	if n == 0 && err == nil {
		err = io.ErrNoProgress
	}
	return b.oneByte[0], err
}

// ReadFrom reads from the provided Reader, placing the bytes in the write area.
func (b *BytesBuffer) ReadFrom(r io.Reader) (int64, error) {
	n, err := r.Read(b.data[b.ri:])
	if err == nil {
		b.wi += n
	}
	return int64(n), err
}

func (b *BytesBuffer) UnreadByte() error {
	if can := b.wi - b.ri; can > 0 {
		b.wi -= 1
		return nil
	}
	return io.ErrNoProgress
}

func (b *BytesBuffer) AsyncReadFrom(r AsyncReader, cb AsyncCallback) {
	r.AsyncRead(b.data[b.ri:], func(err error, n int) {
		if err == nil {
			b.wi += n
		}
		cb(err, n)
	})
}

func (b *BytesBuffer) Write(bb []byte) (int, error) {
	b.data = append(b.data, bb...)
	n := len(bb)
	b.wi += n
	return n, nil
}

func (b *BytesBuffer) WriteByte(bb byte) error {
	b.data = append(b.data, bb)
	b.wi += 1
	return nil
}

func (b *BytesBuffer) WriteString(s string) (int, error) {
	b.data = append(b.data, s...)
	n := len(s)
	b.wi += n
	return n, nil
}

func (b *BytesBuffer) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(b.data[:b.ri])
	if err == nil {
		b.Consume(n)
	}
	return int64(n), err
}

func (b *BytesBuffer) AsyncWriteTo(w AsyncWriter, cb AsyncCallback) {
	w.AsyncWriteAll(b.data[:b.ri], func(err error, n int) {
		if err == nil {
			b.Consume(n)
		}
		cb(err, n)
	})
}
