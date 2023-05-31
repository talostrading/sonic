package sonic

type SlotManager struct {
	sequencer *SlotSequencer
	offsetter *SlotOffsetter
}

func NewSlotManager(nSlots, maxBytes int) *SlotManager {
	s := &SlotManager{}
	s.sequencer = NewSlotSequencer(nSlots, maxBytes)
	s.offsetter = NewSlotOffsetter(maxBytes)
	return s
}

func (s *SlotManager) Push(seq int, slot Slot) (bool, error) {
	slot = s.offsetter.Add(slot)
	return s.sequencer.Push(seq, slot)
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
