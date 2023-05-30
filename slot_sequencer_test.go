package sonic

import (
	"testing"
)

func setupSlotSequencerTest(
	t *testing.T, xs []byte,
) (*SlotSequencer,
	*ByteBuffer) {
	s, err := NewSlotSequencer(len(xs))
	if err != nil {
		t.Fatal(err)
	}
	b := NewByteBuffer()

	n, err := b.Write(xs)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(xs) {
		t.Fatal("invalid write")
	}

	b.Commit(len(xs))

	return s, b
}

func checkSlotSequencer(t *testing.T, s *SlotSequencer, n int) {
	last := s.slots[0].seq
	for i := 1; i < len(s.slots); i++ {
		if s.slots[i].seq <= last {
			t.Fatal("wrong sequencing")
		}
	}
	if s.Size() != n {
		t.Fatal("wrong size")
	}
}

func TestSlotSequencerPush0(t *testing.T) {
	permutation := []byte{0}
	s, b := setupSlotSequencerTest(t, permutation)
	defer checkSlotSequencer(t, s, len(permutation))

	for i := 0; i < len(permutation); i++ {
		pushed := s.Push(int(permutation[i]), b.Save(1))
		if !pushed {
			t.Fatal("not pushed")
		}
	}
}

func TestSlotSequencerPush1(t *testing.T) {
	permutation := []byte{0, 1, 2, 3, 4}
	s, b := setupSlotSequencerTest(t, permutation)
	defer checkSlotSequencer(t, s, len(permutation))

	for i := 0; i < len(permutation); i++ {
		pushed := s.Push(int(permutation[i]), b.Save(1))
		if !pushed {
			t.Fatal("not pushed")
		}
	}
}

func TestSlotSequencerPush2(t *testing.T) {
	permutation := []byte{4, 2, 0, 1, 3}
	s, b := setupSlotSequencerTest(t, permutation)
	defer checkSlotSequencer(t, s, len(permutation))

	for i := 0; i < len(permutation); i++ {
		pushed := s.Push(int(permutation[i]), b.Save(1))
		if !pushed {
			t.Fatal("not pushed")
		}
	}

}

func TestSlotSequencerPush3(t *testing.T) {
	permutation := []byte{4, 2, 0, 1, 3}
	s, b := setupSlotSequencerTest(t, permutation)
	defer checkSlotSequencer(t, s, len(permutation))

	var slots []Slot

	for i := 0; i < len(permutation); i++ {
		slot := b.Save(1)
		slots = append(slots, slot)
		pushed := s.Push(int(permutation[i]), slot)
		if !pushed {
			t.Fatal("not pushed")
		}
	}

	for j := 0; j < 1000; j++ {
		for i, slot := range slots {
			pushed := s.Push(int(permutation[i]), slot)
			if pushed {
				t.Fatal("pushed")
			}
		}
	}

	if s.Size() != len(permutation) {
		t.Fatal("wrong size")
	}
}

func TestSlotSequencerPop0(t *testing.T) {
	s, b := setupSlotSequencerTest(t, []byte{0, 1, 2, 3, 4})
	for seq := 0; seq < 5; seq++ {
		s.Push(seq, b.Save(1))
	}

	for seq := 0; seq < 5; seq++ {
		slot, ok := s.Pop(seq)
		if !ok {
			t.Fatal("not popped")
		}
		if slot.Length != 1 || slot.Index != 0 {
			t.Fatal("wrong slot")
		}
	}

	if s.Size() != 0 {
		t.Fatal("wrong size")
	}
}

func TestSlotSequencerPop1(t *testing.T) {
	// Pop in sequenced order. That means each slot following a popped one must
	// be offset on the next pop. That also means each popped slot has Index 0.

	s, b := setupSlotSequencerTest(t, []byte{0, 1, 2, 3, 4})
	for seq := 10; seq < 15; seq++ {
		s.Push(seq, b.Save(1))
	}

	for seq := 10; seq < 15; seq++ {
		slot, _ := s.Pop(seq)
		if slot.Index != 0 {
			t.Fatal("wrong index for slot")
		}
		discarded := b.Discard(slot)
		if slot.Length != discarded {
			t.Fatal("wrong Discard return")
		}
	}

	if s.Size() != 0 {
		t.Fatal("slot sequencer should be empty")
	}
}

func TestSlotSequencerPop2(t *testing.T) {
	// Pop in reverse sequenced order. That means none of the slots will be
	// offset.

	s, b := setupSlotSequencerTest(t, []byte{0, 1, 2, 3, 4})
	for seq := 10; seq < 15; seq++ {
		s.Push(seq, b.Save(1))
	}

	expectedIndex := 4
	for seq := 14; seq >= 10; seq-- {
		slot, _ := s.Pop(seq)
		if slot.Index != expectedIndex {
			t.Fatal("wrong index for slot")
		}
		expectedIndex--
		discarded := b.Discard(slot)
		if slot.Length != discarded {
			t.Fatal("wrong Discard return")
		}
	}

	if s.Size() != 0 {
		t.Fatal("slot sequencer should be empty")
	}
}

func TestSlotSequencerPop3(t *testing.T) {
	// Pop randomly.

	s, b := setupSlotSequencerTest(t, []byte{0, 1, 2, 3, 4})
	for seq := 10; seq < 15; seq++ {
		s.Push(seq, b.Save(1))
	}

	permutation := []int{11, 13, 10, 12, 14}
	//	{ 1,  3,  0,  2,  4},      none popped
	//	{-1,  2,  0,  1,  3},      11 popped. 12, 13, 14 offset by 1 = 11.Length
	//	{-1, -1,  0,  1,  2},      13 popped. 14 offset by 1 = 13.Length
	//	{-1, -1, -1,  0,  1},      10 popped. 12, 14 offset by 1 = 10.Length
	//	{-1, -1, -1, -1,  0},      12 popped. 14 offset by 1 = 12.Length
	//	{-1, -1, -1, -1, -1},      14 popped. done.
	expectedIndices := []int{1, 2, 0, 0, 0}
	for i := 0; i < len(permutation); i++ {
		seq := permutation[i]
		slot, _ := s.Pop(seq)
		discarded := b.Discard(slot)
		if slot.Length != discarded {
			t.Fatal("wrong Discard return")
		}
		if slot.Index != expectedIndices[i] {
			t.Fatalf(
				"wrong index given=%d expected=%d",
				slot.Index, expectedIndices[i])
		}
	}

	if s.Size() != 0 {
		t.Fatal("slot sequencer should be empty")
	}
}

func TestSlotSequencerPopRange0(t *testing.T) {
	// PopRange(0, 2) on seq[0, 1, 2, 3, 4] => seq[2, 3, 4]

	s, b := setupSlotSequencerTest(t, []byte{0, 1, 2, 3, 4})
	for i := 0; i < 5; i++ {
		s.Push(i, b.Save(1))
	}

	popped := s.PopRange(0, 2)
	if len(popped) != 2 {
		t.Fatal("wrong pop range")
	}
	if len(s.slots) != 3 {
		t.Fatal("wrong pop range")
	}
	for i := 0; i < len(popped); i++ {
		if popped[i].Index != i || popped[i].Length != 1 {
			t.Fatal("wrong pop range")
		}
	}
	for i := 0; i < 3; i++ {
		if s.slots[i].Index != len(popped)+i || s.slots[i].Length != 1 {
			t.Fatal("wrong pop range")
		}
	}
}

func TestSlotSequencerPopRange1(t *testing.T) {
	// PopRange(0, 2) on seq[1, 2, 3, 4] => seq[2, 3, 4]

	s, b := setupSlotSequencerTest(t, []byte{0, 1, 2, 3, 4})
	for i := 0; i < 5; i++ {
		s.Push(i, b.Save(1))
	}

	_, ok := s.Pop(0)
	if !ok {
		t.Fatal("not popped")
	}

	popped := s.PopRange(0, 2)
	if len(popped) != 1 {
		t.Fatal("wrong pop range")
	}
	if len(s.slots) != 3 {
		t.Fatal("wrong pop range")
	}
	for i := 0; i < len(popped); i++ {
		if popped[i].Index != 1+i || popped[i].Length != 1 {
			t.Fatal("wrong pop range")
		}
	}
	for i := 0; i < 3; i++ {
		if s.slots[i].Index != 2+i || s.slots[i].Length != 1 {
			t.Fatal("wrong pop range")
		}
	}
}

func TestSlotSequencerPopRange2(t *testing.T) {
	// PopRange(0, 100) on seq[1, 2, 3, 4] => seq[]

	s, b := setupSlotSequencerTest(t, []byte{0, 1, 2, 3, 4})
	for i := 0; i < 5; i++ {
		s.Push(i, b.Save(1))
	}

	popped := s.PopRange(0, 100)
	if len(popped) != 5 {
		t.Fatal("wrong pop range")
	}
	if len(s.slots) != 0 {
		t.Fatal("wrong pop")
	}
}

func TestSlotSequencerPopRange3(t *testing.T) {
	// PopRange(0, 1) on seq[0, 1, 2, 3, 4] => seq[1, 2, 3, 4]

	s, b := setupSlotSequencerTest(t, []byte{0, 1, 2, 3, 4})
	for i := 0; i < 5; i++ {
		s.Push(i, b.Save(1))
	}

	popped := s.PopRange(0, 1)
	if len(popped) != 1 {
		t.Fatal("wrong pop range")
	}
	if len(s.slots) != 4 {
		t.Fatal("wrong pop")
	}
}
