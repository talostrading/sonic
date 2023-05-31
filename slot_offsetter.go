package sonic

import "github.com/talostrading/sonic/util"

type SlotOffsetter struct {
	tree *util.FenwickTree
}

func NewSlotOffsetter(maxBytes int) *SlotOffsetter {
	s := &SlotOffsetter{}
	s.tree = util.NewFenwickTree(maxBytes)
	return s
}

func (s *SlotOffsetter) Offset(slot Slot) Slot {
	offset := s.tree.SumUntil(slot.Index)
	s.tree.Add(slot.Index, slot.Length)
	return OffsetSlot(offset, slot)
}

func (s *SlotOffsetter) Clear() {
	s.tree.ClearAll()
}

func (s *SlotOffsetter) Add(slot Slot) (Slot, error) {
	slot.Index += s.tree.Sum()
	if slot.Index >= s.tree.Size() {
		return Slot{}, ErrNoSpaceLeftForSlot
	}
	return slot, nil
}
