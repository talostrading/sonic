package websocket

// FrameAssembler helps users reassemble a set of payload slices (frames)
// into a single contiguous buffer.
type FrameAssembler struct {
	parts [][]byte
}

// NewFrameAssembler creates a FrameAssembler with the given parts.
func NewFrameAssembler(parts ...[]byte) *FrameAssembler {
	return &FrameAssembler{
		parts: parts,
	}
}

func (fa *FrameAssembler) Append(fragment []byte) {
	fa.parts = append(fa.parts, fragment)
}

// Slices returns the underlying slices exactly as stored.
func (fa *FrameAssembler) Slices() [][]byte {
	return fa.parts
}

// Reassemble concatenates all slices into a single new (allocated) []byte buffer.
func (fa *FrameAssembler) Reassemble() []byte {
	total := 0
	for _, p := range fa.parts {
		total += len(p)
	}
	out := make([]byte, total)
	offset := 0
	for _, p := range fa.parts {
		copy(out[offset:], p)
		offset += len(p)
	}
	return out
}

// ReassembleInto concatenates all slices into the provided []byte buffer.
func (fa *FrameAssembler) ReassembleInto(b []byte) {
	offset := 0
	for _, p := range fa.parts {
		copy(b[offset:], p)
		offset += len(p)
	}
}
