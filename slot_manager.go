package sonic

type SlotManager struct {
	sequencer *SlotSequencer
	offsetter *SlotOffsetter
}

func NewSlotManager(maxSlots, maxBytes int) *SlotManager {
	s := &SlotManager{}
	s.sequencer = NewSlotSequencer(maxSlots)
	s.offsetter = NewSlotOffsetter(maxBytes)
	return s
}

func (s *SlotManager) Push(seq int, slot Slot) (ok bool, err error) {
	slot, err = s.offsetter.Add(slot)
	if err == nil {
		return s.sequencer.Push(seq, slot)
	}
	return false, err
}

func (s *SlotManager) Pop(seq int) (Slot, bool) {
	slot, ok := s.sequencer.Pop(seq)
	if ok {
		slot = s.offsetter.Offset(slot)

		if s.sequencer.Size() == 0 {
			s.offsetter.Clear()
		}
	}
	return slot, ok
}
