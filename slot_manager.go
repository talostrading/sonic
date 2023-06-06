package sonic

type SlotManager struct {
	maxBytes int

	bytes     int
	sequencer *SlotSequencer
	offsetter *SlotOffsetter
}

func NewSlotManager(maxSlots, maxBytes int) *SlotManager {
	s := &SlotManager{
		maxBytes: maxBytes,
	}
	s.sequencer = NewSlotSequencer(maxSlots)
	s.offsetter = NewSlotOffsetter(maxBytes)
	return s
}

func (s *SlotManager) Push(seq int, slot Slot) (ok bool, err error) {
	if s.bytes+slot.Length > s.maxBytes {
		return false, ErrNoSpaceLeftForSlot
	}

	slot, err = s.offsetter.Add(slot)
	if err == nil {
		ok, err = s.sequencer.Push(seq, slot)
		if err == nil {
			s.bytes += slot.Length
		}
	}
	return ok, err
}

func (s *SlotManager) Pop(seq int) (Slot, bool) {
	slot, ok := s.sequencer.Pop(seq)
	if ok {
		slot = s.offsetter.Offset(slot)

		if s.sequencer.Size() == 0 {
			s.offsetter.Clear()
		}

		s.bytes -= slot.Length
	}
	return slot, ok
}

func (s *SlotManager) Size() int {
	return s.sequencer.Size()
}

func (s *SlotManager) Bytes() int {
	return s.bytes
}

func (s *SlotManager) MaxBytes() int {
	return s.maxBytes
}

func (s *SlotManager) FillPct() float64 {
	a := float64(s.Bytes())
	b := float64(s.MaxBytes())
	return a / b * 100.0
}

func (s *SlotManager) Clear() {
	s.offsetter.Clear()
	s.sequencer.Clear()
	s.bytes = 0
}
