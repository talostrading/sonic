package websocket

// FrameAssembler helps users reassemble a set of payload slices (frames)
// into a single contiguous buffer.
type FrameAssembler struct {
	parts [][]byte
	totalLen int
}

// NewFrameAssembler creates a FrameAssembler with the given parts.
func NewFrameAssembler(parts ...[]byte) *FrameAssembler {
	total := 0
	for _, p := range parts {
		total += len(p)
	}

	return &FrameAssembler{
		parts: parts,
		totalLen: total,
	}
}

func (fa *FrameAssembler) Append(fragment []byte) {
	fa.parts = append(fa.parts, fragment)
	fa.totalLen += len(fragment)
}

// Slices returns the underlying slices exactly as stored.
func (fa *FrameAssembler) Slices() [][]byte {
	return fa.parts
}

// Length returns the total length of the payload slices
func (fa *FrameAssembler) Length() int {
	return fa.totalLen
}

// Reassemble concatenates all slices into a single new (allocated) []byte buffer.
func (fa *FrameAssembler) Reassemble() []byte {
	out := make([]byte, fa.totalLen)
	offset := 0
	for _, p := range fa.parts {
		copy(out[offset:], p)
		offset += len(p)
	}

	return out
}

// ReassembleInto concatenates all slices into the provided []byte buffer.
//
// Returns true on success, or false if slices won't fit into the 
// buffer (leaving it untouched).
func (fa *FrameAssembler) ReassembleInto(b []byte) bool {
	if len(b) < fa.totalLen {
		return false
	}

	offset := 0
	for _, p := range fa.parts {
		copy(b[offset:], p)
		offset += len(p)
	}

	return true
}
