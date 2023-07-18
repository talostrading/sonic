package sonic

// SlotSequencer does two things:
// 1. Provides ordering to ByteBuffer.Slots
// 2. Offsets ByteBuffer.Slots such that they are discarded correctly. See
// slot_offsetter.go for a description of why this is necessary and how it's
// done.
//
// Workflow:
// - slot := ByteBuffer.Save(...)
// - slot = sequencer.Push(x, slot) where x is some sequence number of this Slot.
// - ...
// - ByteBuffer.Discard(sequencer.Pop(slot))

type SlotSequencer struct {
	maxBytes int

	bytes     int
	container *sequencedSlots
	offsetter *SlotOffsetter
}

func NewSlotSequencer(maxSlots, maxBytes int) *SlotSequencer {
	s := &SlotSequencer{
		maxBytes: maxBytes,
	}
	s.container = newSequencedSlots(maxSlots)
	s.offsetter = NewSlotOffsetter(maxBytes)
	return s
}

// Push a Slot that's uniquely identified and ordered by `seq`.
func (s *SlotSequencer) Push(seq int, slot Slot) (ok bool, err error) {
	if s.bytes+slot.Length > s.maxBytes {
		return false, ErrNoSpaceLeftForSlot
	}

	slot, err = s.offsetter.Add(slot)
	if err == nil {
		ok, err = s.container.Push(seq, slot)
		if ok && err == nil {
			s.bytes += slot.Length
		}
	}
	return ok, err
}

// Pop the slot identified by `seq`. The popped Slot must be discarded through
// ByteBuffer.Discard before Pop is called again.
func (s *SlotSequencer) Pop(seq int) (Slot, bool) {
	slot, ok := s.container.Pop(seq)
	if ok {
		slot = s.offsetter.Offset(slot)

		if s.container.Size() == 0 {
			s.offsetter.Reset()
		}

		s.bytes -= slot.Length
	}
	return slot, ok
}

func (s *SlotSequencer) Size() int {
	return s.container.Size()
}

func (s *SlotSequencer) Bytes() int {
	return s.bytes
}

func (s *SlotSequencer) MaxBytes() int {
	return s.maxBytes
}

func (s *SlotSequencer) FillPct() float64 {
	a := float64(s.Bytes())
	b := float64(s.MaxBytes())
	return a / b * 100.0
}

func (s *SlotSequencer) Reset() {
	s.offsetter.Reset()
	s.container.Reset()
	s.bytes = 0
}
