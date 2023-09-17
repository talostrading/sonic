package sonic

import (
	"io"

	"github.com/talostrading/sonic/sonicerrors"
)

// ByteBuffer provides operations that make it easier to handle byte slices in
// networking code.
//
// A ByteBuffer has 3 areas. In order:
// - save area:  [0, si)
// - read area:  [si, ri)
// - write area: [ri, wi)
//
// A usual workflow is as follows:
//   - Bytes are written to the write area. These bytes cannot be read yet.
//   - Bytes from the write area are made available for reading in the read area,
//     by calling Commit.
//   - Bytes from the read area can be either Saved or Consumed. If Saved, the
//     bytes are kept in the save area. If Consumed, the bytes' lifetime ends,
//     they are automatically discarded. Saved bytes must be discarded later.
//
// Invariants:
// - 0 <= si <= ri <= wi <= min(len(data), cap(b.data)) <= cap(b.data)
// - everytime wi changes, b.data should grow/shrink accordingly:
//   - b.data = b.data[:b.wi]
type ByteBuffer struct {
	si int // End index of the save area, always smaller or equal to ri.
	ri int // End index of the read area, always smaller or equal to wi.
	wi int // End index of the write area.

	oneByte [1]byte

	data []byte
}

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

// Reserve capacity for at least `n` more bytes to be written
// into the ByteBuffer's write area.
//
// This call grows the write area by at least `n` bytes. This might allocate.
func (b *ByteBuffer) Reserve(n int) {
	existing := cap(b.data) - b.wi
	if need := n - existing; need > 0 {
		b.data = b.data[:cap(b.data)]
		b.data = append(b.data, make([]byte, need)...)
	}
	b.data = b.data[:b.wi]
}

// Reserved returns the number of bytes that can be written
// in the write area of the buffer.
func (b *ByteBuffer) Reserved() int {
	return cap(b.data) - b.wi
}

// Commit moves `n` bytes from the write area to the read area.
func (b *ByteBuffer) Commit(n int) {
	if n <= 0 {
		return
	}

	b.ri += n
	if b.ri > b.wi {
		b.ri = b.wi
	}
}

// Prefault the buffer, forcing physical memory allocation.
//
// NOTE: this should be used sparingly. Even though an array is contiguous in
// the process' virtual memory map, it is probably fragmented in main memory.
// Iterating over the array will cause a bunch of page faults, thus triggering
// virtual to physical memory mapping. This means that if you Reserve 1GB
// initially, you will get nothing allocated. But if you Prefault after Reserve,
// you will get the entire 1GB allocated which is maybe not what you want in a
// resourced constrained application.
func (b *ByteBuffer) Prefault() {
	slice := b.data[:cap(b.data)]
	for i := range slice {
		slice[i] = 0
	}
}

// Data returns the bytes in the read area.
func (b *ByteBuffer) Data() []byte {
	return b.data[b.si:b.ri]
}

// SaveLen returns the length of the save area.
func (b *ByteBuffer) SaveLen() int {
	return len(b.data[0:b.si])
}

// ReadLen returns the length of the read area.
func (b *ByteBuffer) ReadLen() int {
	return len(b.data[b.si:b.ri])
}

// WriteLen returns the length of the write area.
func (b *ByteBuffer) WriteLen() int {
	return len(b.data[b.ri:b.wi])
}

// Len returns the length of the underlying byte slice.
func (b *ByteBuffer) Len() int {
	return len(b.data)
}

// Cap returns the length of the underlying byte slice.
func (b *ByteBuffer) Cap() int {
	return cap(b.data)
}

// Consume removes the first `n` bytes of the read area. The removed bytes
// cannot be referenced after a call to Consume. If that's desired, use Save.
func (b *ByteBuffer) Consume(n int) {
	if n <= 0 {
		return
	}

	if readLen := b.ReadLen(); n > readLen {
		n = readLen
	}

	if n > 0 {
		// TODO this can be smarter
		copy(b.data[b.si:], b.data[b.si+n:b.wi])

		b.ri -= n
		b.wi -= n
		b.data = b.data[:b.wi]
	}
}

// Save n bytes from the read area. Save is like Consume, except that the bytes
// can still be referenced after the read area is updated.
//
// Saved bytes should be discarded at some point with
// Discard(...).
func (b *ByteBuffer) Save(n int) (slot Slot) {
	if readLen := b.ReadLen(); n > readLen {
		n = readLen
	}
	if n <= 0 {
		return
	}
	slot.Length = n
	slot.Index = b.si
	b.si += n
	return
}

// Saved bytes.
func (b *ByteBuffer) Saved() []byte {
	return b.data[:b.si]
}

// SavedSlot ...
func (b *ByteBuffer) SavedSlot(slot Slot) []byte {
	return b.data[slot.Index : slot.Index+slot.Length]
}

// Discard a previously saved slot.
//
// This call reduces the save area by slot.Length. Returns slot.Length.
func (b *ByteBuffer) Discard(slot Slot) (discarded int) {
	if slot.Length <= 0 {
		return 0
	}

	copy(b.data[slot.Index:], b.data[slot.Index+slot.Length:b.wi])
	b.si -= slot.Length
	b.ri -= slot.Length
	b.wi -= slot.Length
	b.data = b.data[:b.wi]

	return slot.Length
}

// DiscardAll saved slots.
//
// The save area's size will be 0 after this call.
func (b *ByteBuffer) DiscardAll() {
	b.Discard(Slot{Index: 0, Length: b.SaveLen()})
}

func (b *ByteBuffer) Reset() {
	b.si = 0
	b.ri = 0
	b.wi = 0
	b.data = b.data[:0]
}

// Read the bytes from the read area into `dst`. Consume them.
func (b *ByteBuffer) Read(dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}

	if b.ri == 0 {
		return 0, io.EOF
	}

	n := copy(dst, b.data[b.si:b.ri])
	b.Consume(n)

	return n, nil
}

// ReadByte returns and consumes one byte from the read area.
func (b *ByteBuffer) ReadByte() (byte, error) {
	_, err := b.Read(b.oneByte[:])
	return b.oneByte[0], err
}

// ReadFrom the supplied reader into the write area.
//
// The buffer is not automatically grown to accommodate all data from the reader.
// The responsibility is left to the caller which can reserve enough space
// through Reserve.
func (b *ByteBuffer) ReadFrom(r io.Reader) (int64, error) {
	n, err := r.Read(b.data[b.wi:cap(b.data)])
	if err == nil {
		b.wi += n
		b.data = b.data[:b.wi]
	}
	return int64(n), err
}

// UnreadByte from the write area.
func (b *ByteBuffer) UnreadByte() error {
	if b.WriteLen() > 0 {
		b.wi -= 1
		b.data = b.data[:b.wi]
		return nil
	}
	return io.EOF
}

// AsyncReadFrom the supplied asynchronous reader into the write area.
//
// The buffer is not automatically grown to accommodate all data from the reader.
// The responsibility is left to the caller which can reserve enough space
// through Reserve.
func (b *ByteBuffer) AsyncReadFrom(r AsyncReader, cb AsyncCallback) {
	r.AsyncRead(b.data[b.wi:cap(b.data)], func(err error, n int) {
		if err == nil {
			b.wi += n
			b.data = b.data[:b.wi]
		}
		cb(err, n)
	})
}

// Write the supplied slice into the write area. Grow the write area if needed.
func (b *ByteBuffer) Write(bb []byte) (int, error) {
	b.data = append(b.data, bb...)
	n := len(bb)
	b.wi += n
	b.data = b.data[:b.wi]
	return n, nil
}

// WriteByte into the write area. Grow the write area if needed.
func (b *ByteBuffer) WriteByte(bb byte) error {
	b.data = append(b.data, bb)
	b.wi += 1
	b.data = b.data[:b.wi]
	return nil
}

// WriteString into the write area. Grow the write area if needed.
func (b *ByteBuffer) WriteString(s string) (int, error) {
	b.data = append(b.data, s...)
	n := len(s)
	b.wi += n
	b.data = b.data[:b.wi]
	return n, nil
}

// WriteTo the provided writer bytes from the read area. Consume them if no
// error occurred.
func (b *ByteBuffer) WriteTo(w io.Writer) (int64, error) {
	var (
		n            int
		err          error
		writtenBytes = 0
	)

	for b.si+writtenBytes < b.ri {
		n, err = w.Write(b.data[b.si+writtenBytes : b.ri])
		if err != nil {
			break
		}
		writtenBytes += n
	}
	b.Consume(writtenBytes)
	return int64(writtenBytes), err
}

// AsyncWriteTo the provided asynchronous writer bytes from the read area.
// Consume them if no error occurred.
func (b *ByteBuffer) AsyncWriteTo(w AsyncWriter, cb AsyncCallback) {
	w.AsyncWriteAll(b.data[b.si:b.ri], func(err error, n int) {
		if err == nil {
			b.Consume(n)
		}
		cb(err, n)
	})
}

// PrepareRead prepares n bytes to be read from the read area. If less than n
// bytes are available, ErrNeedMore is returned and no bytes are committed to
// the read area, hence made available for reading.
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

// Claim a byte slice of the write area.
//
// Claim allows callers to write directly into the write area of the buffer.
//
// `fn` implementations should return the number of bytes written into the
// provided byte slice.
//
// Callers have the option to write less than they claim. The amount is returned
// in the callback and the unused bytes will be used in future claims.
func (b *ByteBuffer) Claim(fn func(b []byte) int) {
	n := fn(b.data[b.wi:cap(b.data)])
	if wi := b.wi + n; n >= 0 && wi <= cap(b.data) {
		// wi <= cap(b.data) because the invariant is that b.wi = min(len(b.data), cap(b.data)) after each call
		b.wi = wi
		b.data = b.data[:b.wi]
	}
}

// ClaimFixed claims a fixed byte slice from the write area.
//
// Callers do not have the option to write less than they claim. The write area
// will grow by `n`.
func (b *ByteBuffer) ClaimFixed(n int) (claimed []byte) {
	if wi := b.wi + n; n >= 0 && wi <= cap(b.data) {
		claimed = b.data[b.wi:wi]
		b.wi = wi
		b.data = b.data[:b.wi]
	}
	return
}

// ShrinkBy shrinks the write area by at most `n` bytes.
func (b *ByteBuffer) ShrinkBy(n int) int {
	if n <= 0 {
		return 0
	}

	if length := b.WriteLen(); n > length {
		n = length
	}
	b.wi -= n
	b.data = b.data[:b.wi]
	return n
}

// ShrinkTo shrinks the write to contain min(n, WriteLen()) bytes.
func (b *ByteBuffer) ShrinkTo(n int) (shrunkBy int) {
	return b.ShrinkBy(b.WriteLen() - n)
}
