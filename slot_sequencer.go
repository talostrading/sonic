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

	bytes   int
	slots   []sequencedSlot // debatable if this is the best data structure
	offsets *util.FenwickTree
}

func NewSlotSequencer(maxBytes int) (*SlotSequencer, error) {
	s := &SlotSequencer{
		maxBytes: maxBytes,
	}

	s.slots = util.ExtendSlice(s.slots, maxBytes)
	s.slots = s.slots[:0]

	s.offsets = util.NewFenwickTree(maxBytes)

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
	// Guard on the Push.
	if slot.Index >= s.offsets.Size() {
		return false, ErrSlotSequencerNoSpace
	}

	// Guard on the Pop.
	offsetIndex := slot.Index + s.offsets.SumUntil(slot.Index)
	if offsetIndex >= s.offsets.Size() {
		return false, ErrSlotSequencerNoSpace
	}

	ix := sort.Search(len(s.slots), func(i int) bool {
		return s.slots[i].seq >= seq
	})
	newSlot := sequencedSlot{
		Slot: Slot{
			// This is a new Slot whose Index can be directly used in its
			// originating ByteBuffer, without any offsetting. This Slot however
			// might come after already offset slots. This means that on Pop,
			// this slot will be offset by n=s.offsets.SumUntil(slot.Index). To
			// deem that offsetting unnecessary, we add n to Index. That means
			// on Pop, the index will be: s.Index + n - n + actual_offset where
			// actual_offset is positive if there were some slots popped before
			// the given Slot.
			Index:  slot.Index + s.offsets.SumUntil(slot.Index),
			Length: slot.Length,
		},
		seq: seq,
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
		offset := s.offsets.SumUntil(slot.Index)
		carry := s.offsets.Clear(slot.Index) + slot.Length
		if next := ix + 1; next < len(s.slots) {
			s.offsets.Add(s.slots[next].Index, carry)
		}
		slot = OffsetSlot(offset, slot)

		s.slots = append(s.slots[:ix], s.slots[ix+1:]...)

		s.bytes -= slot.Length

		return slot, true
	}
	return Slot{}, false
}

// PopRange pops at most n slots that come in sequence, starting from sequence
// number n.
// TODO offsetting in this one
func (s *SlotSequencer) PopRange(seq, n int) (poppedSlots []Slot) {
	panic("use Pop instead, PopRange does not do slot offsetting yet")

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
