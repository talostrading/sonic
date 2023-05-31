package sonic

import (
	"errors"
	"fmt"
	"github.com/talostrading/sonic/util"
	"sort"
)

type sequencedSlot struct {
	Slot
	seq int // sequence number of this slot
}

func (s sequencedSlot) String() string {
	return fmt.Sprintf(
		"[seq=%d (index=%d length=%d)]",
		s.seq, s.Index, s.Length,
	)
}

type SlotSequencer struct {
	maxBytes int

	bytes int
	slots []sequencedSlot // debatable if this is the best data structure
}

func NewSlotSequencer(nSlots, maxBytes int) (*SlotSequencer, error) {
	s := &SlotSequencer{
		maxBytes: maxBytes,
	}

	s.slots = util.ExtendSlice(s.slots, nSlots)
	s.slots = s.slots[:0]

	return s, nil
}

var ErrSlotSequencerNoSpace = errors.New(
	"no space left to buffer in the slot sequencer",
)

// Push a Slot created when saving some bytes to the ByteBuffer's save area. Its
// position in the SlotSequencer is determined by its seq i.e. sequence number.
// The SlotSequencer keeps slots in ascending order of their sequence number
// and assumes that slot_a precedes slot_b if:
// - seq(slot_b) > seq(slot_a) >= 0
// - seq(slot_b) - seq(slot_a) == 1
func (s *SlotSequencer) Push(seq int, slot Slot) (bool, error) {
	// Guard overall.
	if s.bytes+slot.Length > s.maxBytes {
		return false, ErrSlotSequencerNoSpace
	}

	ix := sort.Search(len(s.slots), func(i int) bool {
		return s.slots[i].seq >= seq
	})

	newSlot := sequencedSlot{
		Slot: slot,
		seq:  seq,
	}
	if ix >= len(s.slots) {
		s.slots = append(s.slots, newSlot)
		s.bytes += slot.Length
		return true, nil
	} else if s.slots[ix].seq != seq {
		s.slots = append(s.slots[:ix+1], s.slots[ix:]...)
		s.slots[ix] = newSlot
		s.bytes += slot.Length
		return true, nil
	}
	return false, nil
}

// Pop a slot. The returned Slot must be discarded with ByteBuffer.Discard
// before calling Pop again.
func (s *SlotSequencer) Pop(seq int) (Slot, bool) {
	ix := sort.Search(len(s.slots), func(i int) bool {
		return s.slots[i].seq >= seq
	})
	if ix < len(s.slots) && s.slots[ix].seq == seq {
		slot := s.slots[ix].Slot
		s.bytes -= slot.Length
		s.slots = append(s.slots[:ix], s.slots[ix+1:]...)
		return slot, true
	}
	return Slot{}, false
}

// PopRange pops at most n slots that come in sequence, starting from sequence
// number n.
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

// Size ...
func (s *SlotSequencer) Size() int {
	return len(s.slots)
}

// Bytes returns the number of bytes the slot sequencer currently holds.
func (s *SlotSequencer) Bytes() int {
	return s.bytes
}

// MaxBytes the slot sequencer can hold.
func (s *SlotSequencer) MaxBytes() int {
	return s.maxBytes
}
