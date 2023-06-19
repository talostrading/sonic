package sonic

import "testing"

func TestSequencedSlotsBounds(t *testing.T) {
	s := newSequencedSlots(1)

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

	// this one goes over the max slots so we should get an error here
	ok, err = s.Push(2, Slot{Index: 1, Length: 2})
	if ok || err == nil {
		t.Fatal("pushed")
	}

	// pop 1, should work
	popped, ok := s.Pop(1)
	if !ok {
		t.Fatal("should pop")
	}
	if popped.Index != 0 || popped.Length != 1 {
		t.Fatal("wrong slot")
	}

	if s.Size() != 0 {
		t.Fatal("size should be zero")
	}

	for i := 0; i < 10; i++ {
		_, ok := s.Pop(1)
		if ok {
			t.Fatal("should not pop")
		}
	}
}
