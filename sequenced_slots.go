package sonic

import (
	"fmt"
	"sort"

	"github.com/talostrading/sonic/util"
)

type sequencedSlot struct {
	Slot
	seq int // sequence number of this slot
}

func (s sequencedSlot) String() string {
	return fmt.Sprintf(
		"slot[seq=%d (index=%d length=%d)]",
		s.seq, s.Index, s.Length,
	)
}

type sequencedSlots struct {
	maxSlots int

	slots []sequencedSlot
}

func newSequencedSlots(maxSlots int) *sequencedSlots {
	s := &sequencedSlots{
		maxSlots: maxSlots,
	}

	s.slots = util.ExtendSlice(s.slots, maxSlots)
	s.slots = s.slots[:0]

	return s
}

func (s *sequencedSlots) checkSize(slot Slot) error {
	if len(s.slots) >= s.maxSlots {
		return ErrNoSpaceLeftForSlot
	}
	return nil
}

// Push a Slot created when saving some bytes to the ByteBuffer's save area. Its
// position in the SlotSequencer is determined by its seq i.e. sequence number.
// The SlotSequencer keeps slots in ascending order of their sequence number
// and assumes that slot_a precedes slot_b if:
// - seq(slot_b) > seq(slot_a) >= 0
// - seq(slot_b) - seq(slot_a) == 1
func (s *sequencedSlots) Push(seq int, slot Slot) (bool, error) {
	ix := sort.Search(len(s.slots), func(i int) bool {
		return s.slots[i].seq >= seq
	})

	newSlot := sequencedSlot{
		Slot: slot,
		seq:  seq,
	}
	if ix >= len(s.slots) {
		if err := s.checkSize(slot); err != nil {
			return false, err
		}

		s.slots = append(s.slots, newSlot)
		return true, nil
	} else if s.slots[ix].seq != seq {
		if err := s.checkSize(slot); err != nil {
			return false, err
		}

		s.slots = append(s.slots[:ix+1], s.slots[ix:]...)
		s.slots[ix] = newSlot
		return true, nil
	}

	return false, nil
}

// Pop a slot. The returned Slot must be discarded with ByteBuffer.Discard
// before calling Pop again.
func (s *sequencedSlots) Pop(seq int) (Slot, bool) {
	ix := sort.Search(len(s.slots), func(i int) bool {
		return s.slots[i].seq >= seq
	})
	if ix < len(s.slots) && s.slots[ix].seq == seq {
		slot := s.slots[ix].Slot
		s.slots = append(s.slots[:ix], s.slots[ix+1:]...)
		return slot, true
	}
	return Slot{}, false
}

// PopRange pops at most n slots in order, starting from seq. The returned Slots
// must be discarded by ByteBuffer.Discard before calling PopRange again.
// TODO: offsetting is a bit tricky here.
func (s *sequencedSlots) PopRange(seq, n int) (poppedSlots []Slot) {
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

// Size ...
func (s *sequencedSlots) Size() int {
	return len(s.slots)
}

// Reset...
func (s *sequencedSlots) Reset() {
	s.slots = s.slots[:0]
}
