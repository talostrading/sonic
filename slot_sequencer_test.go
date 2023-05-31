package sonic

import "testing"

func TestSlotSequencerBoundSlots(t *testing.T) {
	s := NewSlotSequencer(1, 1024)

	// push 1 for the first time
	ok, err := s.Push(1, Slot{Index: 0, Length: 1})
	if !ok || err != nil {
		t.Fatal("not pushed")
	}

	// push 1 again; should not be pushed since we already have 1,
	// err must be nil
	ok, err = s.Push(1, Slot{Index: 0, Length: 1})
	if ok {
		t.Fatal("pushed")
	}
	if err != nil {
		t.Fatal("errored")
	}

	ok, err = s.Push(2, Slot{Index: 1, Length: 2})
	if ok || err == nil {
		t.Fatal("pushed")
	}
}
