package sonic

import (
	"bytes"
	"fmt"
	"github.com/talostrading/sonic/sonicerrors"
	"io"
)

// ByteBuffer provides operations that make it easier to handle byte slices in networking code.
//
// A ByteBuffer is split into two regions: read and write. Bytes written by the caller are placed in the write region.
// Bytes from the write region can be read if Committed to the read region. Bytes read from the read region can be Consumed
// after a successful read.
type ByteBuffer struct {
	ri int // End index of the read region, always smaller or equal to wi.
	wi int // End index of the write region, equal to len(data)

	// read region range: [0, ri)
	// write region range: [ri, wi)

	// Underlying byte slice where read and write region are mapped.
	data []byte

	// ReadSlice truncates the returned slice, thus leaving both the returned slice and some extra bytes like `delim`
	// or `\r\n` in the read region. Any call that touches the read region will consume the leftover bytes first.
	leftover int

	oneByte [1]byte
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

func (b *ByteBuffer) consumeLeftover() {
	if b.leftover > 0 {
		b.Consume(b.leftover)
		b.leftover = 0
	}
}

// Reserve capacity for at least `n` more bytes to be written
// into the ByteBuffer.
//
// This call grows the write region by at least `n` bytes.
func (b *ByteBuffer) Reserve(n int) {
	existing := cap(b.data) - b.wi
	if need := n - existing; need > 0 {
		b.data = b.data[:cap(b.data)]
		b.data = append(b.data, make([]byte, need)...)
	}
	b.data = b.data[:b.wi]
}

// Reserved returns the number of bytes that can be written
// in the write region of the buffer.
func (b *ByteBuffer) Reserved() int {
	return cap(b.data) - b.wi
}

// Commit moves `n` bytes from the write region to the read region.
func (b *ByteBuffer) Commit(n int) {
	b.consumeLeftover()

	if n < 0 {
		n = 0
	}

	b.ri += n
	if b.ri > b.wi {
		b.ri = b.wi
	}
}

// CommitAll moves all the bytes from the write region to the read region.
func (b *ByteBuffer) CommitAll() {
	n := len(b.data[b.ri:b.wi])
	b.Commit(n)
}

// Data returns the bytes in the read region.
func (b *ByteBuffer) Data() []byte {
	b.consumeLeftover()

	return b.data[:b.ri]
}

// ReadLen the number of bytes in the read region.
func (b *ByteBuffer) ReadLen() int {
	b.consumeLeftover()

	return len(b.data[:b.ri])
}

// WriteLen the number of bytes in the write region.
func (b *ByteBuffer) WriteLen() int {
	return len(b.data[b.ri:b.wi])
}

// Len the number of bytes in both regions.
func (b *ByteBuffer) Len() int {
	b.consumeLeftover()

	return len(b.data)
}

// Cap returns the capacity of the underlying byte slice.
func (b *ByteBuffer) Cap() int {
	b.consumeLeftover()

	return cap(b.data)
}

// Consume removes `n` bytes from the read region.
func (b *ByteBuffer) Consume(n int) {
	if n > b.ri {
		n = b.ri
	}

	if n > 0 {
		copy(b.data, b.data[n:b.wi])
		b.wi -= n
		b.ri -= n

		b.data = b.data[:b.wi]
	}
}

func (b *ByteBuffer) ConsumeAll() {
	n := len(b.data[:b.ri])
	b.Consume(n)
}

func (b *ByteBuffer) Reset() {
	b.ri = 0
	b.wi = 0
	b.data = b.data[:0]
	b.leftover = 0
}

// Read reads and consumes the bytes from the read region into `dst`.
func (b *ByteBuffer) Read(dst []byte) (int, error) {
	b.consumeLeftover()

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

// ReadByte reads and consumes one byte from the read region.
func (b *ByteBuffer) ReadByte() (byte, error) {
	b.consumeLeftover()

	_, err := b.Read(b.oneByte[:])
	return b.oneByte[0], err
}

// ReadSlice reads until the first occurrence of delim in the read region, returning a slice pointing at the bytes.
// The returned bytes are consumed from the read region after the call returns.
//
// If there are no bytes in the buffer, io.EOF is returned.
// If there are bytes but `delim` is not found, ErrNeedMore is returned.
// In any other case, the slice is returned along with a nil error.
func (b *ByteBuffer) ReadSlice(delim byte) (s []byte, err error) {
	b.consumeLeftover()

	if i := bytes.IndexByte(b.data[:b.ri], delim); i >= 0 {
		s = b.data[:i]
		b.leftover += len(s) + 1 // 1 for the delim
	} else {
		err = sonicerrors.ErrNeedMore
	}

	return
}

// ReadLine reads the next line from the read region, returning a slice pointing at the bytes.
// The returned line is consumed from the read region after the call returns.
//
// If there are no bytes in the buffer, io.EOF is returned.
// If there are bytes but no `\r\n` or `\n` characters, ErrNeedMore is returned.
// In any other case, the next line is returned along with a nil error.
func (b *ByteBuffer) ReadLine() (line []byte, err error) {
	line, err = b.ReadSlice('\n')
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return
}

// ReadFrom reads the data from the supplied reader into the write region of the buffer.
//
// The buffer is not automatically grown to accommodate all data from the reader.
// The caller must reserve enough space in the buffer with a call to Reserve.
func (b *ByteBuffer) ReadFrom(r io.Reader) (int64, error) {
	n, err := r.Read(b.data[b.wi:cap(b.data)])
	if err == nil {
		b.wi += n
		b.data = b.data[:b.wi]
	}
	return int64(n), err
}

// UnreadByte removes the last byte from the read region.
func (b *ByteBuffer) UnreadByte() error {
	b.consumeLeftover()

	if can := b.wi - b.ri; can > 0 {
		b.wi -= 1
		b.data = b.data[:b.wi]
		return nil
	}
	return io.EOF
}

// AsyncReadFrom reads the data from the supplied reader into the write region of the buffer, asynchronously.
//
// The buffer is not automatically grown to accommodate all data from the reader.
// The caller must reserve enough space in the buffer with a call to Reserve.
func (b *ByteBuffer) AsyncReadFrom(r AsyncReader, cb AsyncCallback) {
	r.AsyncRead(b.data[b.wi:cap(b.data)], func(err error, n int) {
		if err == nil {
			b.wi += n
			b.data = b.data[:b.wi]
		}
		cb(err, n)
	})
}

// Write writes the supplied bytes into the buffer's write region, growing the buffer
// as needed to accommodate the new data.
func (b *ByteBuffer) Write(bb []byte) (int, error) {
	b.data = append(b.data, bb...)
	n := len(bb)
	b.wi += n
	b.data = b.data[:b.wi]
	return n, nil
}

// WriteByte writes the supplied byte into the buffer's write region, growing the buffer
// as needed to accommodate the new data.
func (b *ByteBuffer) WriteByte(bb byte) error {
	b.data = append(b.data, bb)
	b.wi += 1
	b.data = b.data[:b.wi]
	return nil
}

// WriteString writes the supplied string into the buffer's write region, growing the buffer
// as needed to accommodate the new data.
func (b *ByteBuffer) WriteString(s string) (int, error) {
	b.data = append(b.data, s...)
	n := len(s)
	b.wi += n
	b.data = b.data[:b.wi]
	return n, nil
}

// WriteTo consumes and writes the bytes from the read region to the provided writer.
func (b *ByteBuffer) WriteTo(w io.Writer) (int64, error) {
	b.consumeLeftover()

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

// AsyncWriteTo writes all the bytes from the read region to the provided writer asynchronously.
func (b *ByteBuffer) AsyncWriteTo(w AsyncWriter, cb AsyncCallback) {
	b.consumeLeftover()

	w.AsyncWriteAll(b.data[:b.ri], func(err error, n int) {
		b.Consume(n)
		cb(err, n)
	})
}

// PrepareRead prepares n bytes to be read from the read region. If less than n bytes are
// available, ErrNeedMore is returned and no bytes are committed to the read region.
func (b *ByteBuffer) PrepareRead(n int) (err error) {
	b.consumeLeftover()

	if need := n - b.ReadLen(); need > 0 {
		if b.WriteLen() >= need {
			b.Commit(need)
		} else {
			err = sonicerrors.ErrNeedMore
		}
	}
	return
}

func (b *ByteBuffer) PrepareReadSlice(delim byte) (err error) {
	b.consumeLeftover()

	if i := bytes.IndexByte(b.data[b.ri:b.wi], delim); i >= 0 {
		b.Commit(i + 1)
	} else {
		err = sonicerrors.ErrNeedMore
	}
	return
}

func (b *ByteBuffer) PrepareReadLine() (err error) {
	return b.PrepareReadSlice('\n')
}

func (b *ByteBuffer) NextLine() (line []byte, err error) {
	if b.ri > b.leftover {
		return nil, fmt.Errorf("can only provide next line if the read region is empty")
	}

	if err = b.PrepareReadLine(); err != nil {
		return nil, err
	}

	return b.ReadLine()
}
