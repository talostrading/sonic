package sonic

import (
	"fmt"
	"github.com/talostrading/sonic/util"
	"sort"
)

type sequencedSlot struct {
	Slot

	// Sequence number of this slot.
	seq int

	// nth saved slot in the ByteBuffer save area. This is taken in Push as the
	// assumed workflow is: sequencer.Push(seq, buffer.Save(n)).
	//
	// Might need to be changed if the SlotSequencer keeps a reference to the
	// ByteBuffer and the workflow changes.
	nth int
}

func (s sequencedSlot) String() string {
	return fmt.Sprintf(
		"[index=%d length=%d seq=%d nth=%d]", s.Index,
		s.Length, s.seq, s.nth,
	)
}

type SlotSequencer struct {
	slots   []sequencedSlot // debatable if this is the best data structure
	offsets *util.FenwickTree
}

func NewSlotSequencer(n int) (*SlotSequencer, error) {
	s := &SlotSequencer{}

	s.slots = util.ExtendSlice(s.slots, n)
	s.slots = s.slots[:0]

	s.offsets = util.NewFenwickTree(n)

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
			nth:  len(s.slots),
		})
		return true
	} else if s.slots[ix].seq != seq {
		s.slots = append(s.slots[:ix+1], s.slots[ix:]...)
		s.slots[ix] = sequencedSlot{
			Slot: slot,
			seq:  seq,
			nth:  ix,
		}
		return true
	}
	return false
}

func (s *SlotSequencer) offset(nth int, slot Slot) Slot {
	slot = OffsetSlot(s.offsets.SumUntil(nth), slot)
	s.offsets.Add(nth, slot.Length)
	return slot
}

// Pop a slot. The returned Slot must be discarded with ByteBuffer.Discard
// before calling Pop(...) again.
func (s *SlotSequencer) Pop(seq int) (Slot, bool) {
	ix := sort.Search(len(s.slots), func(i int) bool {
		return s.slots[i].seq >= seq
	})
	if ix < len(s.slots) && s.slots[ix].seq == seq {
		slot := s.slots[ix]
		s.slots = append(s.slots[:ix], s.slots[ix+1:]...)
		return s.offset(slot.nth, slot.Slot), true
	}
	return Slot{}, false
}

func (s *SlotSequencer) PopRange(seq, n int) (poppedSlots []Slot) {
	if n > len(s.slots) {
		n = len(s.slots)
	}

	if n == 0 {
		return nil
	}

	ix := sort.Search(len(s.slots), func(i int) bool {
		return s.slots[i].seq >= seq
	})
	if ix < len(s.slots) {
		// PopRange(0, 2) on seq[0, 1, 2, 3] => [2, 3]
		// PopRange(0, 2) on seq[1, 2, 3] => [2, 3]
		//   - here we want to pop 2 starting from sequence number 0
		//   - there is no sequence number zero, and the closest one is 1
		//   - hence we consider 0 already popped, and we must only pop 1 now
		//   - that's what toPop accounts for
		toPop := n - (s.slots[ix].seq - seq)

		poppedSlots = util.ExtendSlice(poppedSlots, toPop)
		poppedSlots = poppedSlots[:0]

		lastSeq := -1
		for i := 0; i < toPop; i++ {
			maybePoppedSlot := s.slots[ix+i]
			if lastSeq == -1 || maybePoppedSlot.seq-lastSeq == 1 {
				lastSeq = maybePoppedSlot.seq
				poppedSlots = append(poppedSlots, maybePoppedSlot.Slot)
			} else {
				break
			}
		}
		s.slots = append(s.slots[:ix], s.slots[ix+toPop:]...)
		return poppedSlots
	}
	return nil
}

func (s *SlotSequencer) Size() int {
	return len(s.slots)
}
