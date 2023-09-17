package sonic

// BipBuffer is a circular buffer capable of providing continuous, arbitrarily
// sized byte chunks in a first-in-first-out manner.
//
// A BipBuffer is useful when writing packet-based network protocols, as it
// allows one to access the packets in a first-in-first-out manner without
// allocations or copies. Callers are always provided a full packet and never
// have to care about wrapped byte chunks.
//
// The usual workflow is as follows:
// - b = buf.Claim(128)
// - n = reader.Read(b)
// - buf.Commit(n)
// - b = buf.Claim(128)
// - n = reader.Read(b)
// - buf.Commit(n)
// - buf.Consume(128) gets rid of the first 128 bytes
// - buf.Consume(128) gets rid of the last 128 bytes
//
// Copyright: Simon Cooke.
type BipBuffer struct {
	data                     []byte
	head, tail               int
	wrappedHead, wrappedTail int
	claimHead, claimTail     int
}

func NewBipBuffer(n int) *BipBuffer {
	b := &BipBuffer{
		data: make([]byte, n),
	}
	return b
}

// Prefault the buffer, forcing physical memory allocation.
func (buf *BipBuffer) Prefault() {
	for i := range buf.data {
		buf.data[i] = 0
	}
}

// Reset the buffer. All committed state is lost.
func (buf *BipBuffer) Reset() {
	buf.head = 0
	buf.tail = 0
	buf.wrappedHead = 0
	buf.wrappedTail = 0
	buf.claimHead = 0
	buf.claimTail = 0
}

func (buf *BipBuffer) Wrapped() bool {
	return buf.wrappedTail-buf.wrappedHead > 0
}

// Claim n bytes.
func (buf *BipBuffer) Claim(n int) []byte {
	var (
		claimHead int
		freeSpace int
	)
	if buf.Wrapped() {
		claimHead = buf.wrappedTail
		freeSpace = buf.head - buf.wrappedTail
	} else {
		spaceBefore := buf.head
		spaceAfter := buf.Size() - buf.tail
		if spaceBefore <= spaceAfter {
			claimHead = buf.tail
			freeSpace = spaceAfter
		} else {
			claimHead = 0
			freeSpace = spaceBefore
		}
	}
	if freeSpace == 0 {
		return nil
	}
	claimSize := freeSpace
	if claimSize > n {
		claimSize = n
	}
	buf.claimHead = claimHead
	buf.claimTail = claimHead + claimSize
	return buf.data[buf.claimHead:buf.claimTail]
}

// Commit n bytes of the previously claimed slice. Returns the committed chunk
// at the tail of the buffer.
func (buf *BipBuffer) Commit(n int) []byte {
	if n == 0 {
		buf.claimHead = 0
		buf.claimTail = 0
		return nil
	}
	toCommit := buf.claimTail - buf.claimHead
	if toCommit > n {
		toCommit = n
	}
	var head, tail int
	if buf.Committed() == 0 {
		buf.head = buf.claimHead
		buf.tail = buf.claimHead + toCommit
		head = buf.head
		tail = buf.tail
	} else if buf.claimHead == buf.tail {
		head = buf.tail
		buf.tail += toCommit
		tail = buf.tail
	} else {
		head = buf.wrappedTail
		buf.wrappedTail += toCommit
		tail = buf.wrappedTail
	}
	buf.claimHead = 0
	buf.claimTail = 0
	return buf.data[head:tail]
}

// Head returns the first (and possibly only) contiguous byte slice in the
// buffer.
func (buf *BipBuffer) Head() []byte {
	if buf.tail-buf.head > 0 {
		return buf.data[buf.head:buf.tail]
	}
	return nil
}

// Consume n bytes from the head of the buffer. This means that the first n
// bytes will get overwritten by a Claim + Commit at some point.
func (buf *BipBuffer) Consume(n int) {
	if n >= buf.tail-buf.head {
		buf.head = buf.wrappedHead
		buf.tail = buf.wrappedTail
		buf.wrappedHead = 0
		buf.wrappedTail = 0
	} else {
		buf.head += n
	}
}

// Committed returns the number of used bytes in the buffer.
func (buf *BipBuffer) Committed() int {
	return buf.tail - buf.head + buf.wrappedTail - buf.wrappedHead
}

// Claimed returns the number of claimed bytes in the buffer.
func (buf *BipBuffer) Claimed() int {
	return buf.claimTail - buf.claimHead
}

// Size of the buffer.
func (buf *BipBuffer) Size() int {
	return len(buf.data)
}

// Empty ...
func (buf *BipBuffer) Empty() bool {
	return buf.Claimed() == 0 && buf.Committed() == 0
}
