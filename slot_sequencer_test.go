package sonic

import "testing"

func setupSlotSequencerTest(
	t *testing.T, xs []byte,
) (*SlotSequencer,
	*ByteBuffer) {
	s, err := NewSlotSequencer()
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

func checkSlotSequencer(t *testing.T, s *SlotSequencer) {
	last := s.slots[0].seq
	for i := 1; i < len(s.slots); i++ {
		if s.slots[i].seq <= last {
			t.Fatal("wrong sequencing")
		}
	}
}

func TestSlotSequencer0(t *testing.T) {
	permutation := []byte{0}
	s, b := setupSlotSequencerTest(t, permutation)
	defer checkSlotSequencer(t, s)

	for i := 0; i < len(permutation); i++ {
		pushed := s.Push(int(permutation[i]), b.Save(1))
		if !pushed {
			t.Fatal("not pushed")
		}
	}
}

func TestSlotSequencer1(t *testing.T) {
	permutation := []byte{0, 1, 2, 3, 4}
	s, b := setupSlotSequencerTest(t, permutation)
	defer checkSlotSequencer(t, s)

	for i := 0; i < len(permutation); i++ {
		pushed := s.Push(int(permutation[i]), b.Save(1))
		if !pushed {
			t.Fatal("not pushed")
		}
	}
}

func TestSlotSequencer2(t *testing.T) {
	permutation := []byte{4, 2, 0, 1, 3}
	s, b := setupSlotSequencerTest(t, permutation)
	defer checkSlotSequencer(t, s)

	for i := 0; i < len(permutation); i++ {
		pushed := s.Push(int(permutation[i]), b.Save(1))
		if !pushed {
			t.Fatal("not pushed")
		}
	}
}

func TestSlotSequencer3(t *testing.T) {
	permutation := []byte{4, 2, 0, 1, 3}
	s, b := setupSlotSequencerTest(t, permutation)
	defer checkSlotSequencer(t, s)

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
}
