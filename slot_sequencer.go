package sonic

import "sort"

type sequencedSlot struct {
	Slot
	seq int
}

type SlotSequencer struct {
	slots   []sequencedSlot // debatable if this is the best data structure
	offsets []int
}

func NewSlotSequencer() (*SlotSequencer, error) {
	s := &SlotSequencer{}
	return s, nil
}

func (s *SlotSequencer) Push(seq int, slot Slot) bool {
	ix := sort.Search(len(s.slots), func(i int) bool {
		return s.slots[i].seq >= seq
	})
	if ix >= len(s.slots) {
		s.slots = append(s.slots, sequencedSlot{
			Slot: slot,
			seq:  seq,
		})
		return true
	} else if s.slots[ix].seq != seq {
		s.slots = append(s.slots[:ix+1], s.slots[ix:]...)
		s.slots[ix] = sequencedSlot{
			Slot: slot,
			seq:  seq,
		}
		return true
	} else {
		return false
	}
}

func (s *SlotSequencer) Pop(seq int) (Slot, bool) {
	ix := sort.Search(len(s.slots), func(i int) bool {
		return s.slots[i].seq >= seq
	})
	if ix < len(s.slots) && s.slots[ix].seq == seq {
		slot := s.slots[ix]
		s.slots = append(s.slots[:ix], s.slots[ix+1:]...)
		return slot.Slot, true
	}
	return Slot{}, false
}

func (s *SlotSequencer) PopRange(sequenceNumber, n int) []Slot {
	return nil
}

func (s *SlotSequencer) Size() int {
	return len(s.slots)
}
