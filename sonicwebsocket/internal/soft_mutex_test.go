package internal

import (
	"testing"
)

type testOp struct {
	id int
}

func (t testOp) ID() int {
	return t.id
}

var _ operation = testOp{}

var op1 = &testOp{1}
var op2 = &testOp{2}

func TestMutex(t *testing.T) {
	s := &softMutex{}

	s.Lock(op1)
	if s.id != 1 {
		t.Fatalf("wrong id expected=%d given=%d", op1.id, s.id)
	}

	s.Unlock(op1)
	if s.id != 0 {
		t.Fatalf("wrong id expected=%d given=%d", 0, s.id)
	}

	if ok := s.TryLock(op1); !ok {
		t.Fatalf("should have locked")
	} else if s.id != op1.id {
		t.Fatalf("wrong id expected=%d given=%d", op1.id, s.id)
	}

	if ok := s.TryLock(op2); ok {
		t.Fatalf("should not have locked")
	} else if s.id != op1.id {
		t.Fatalf("wrong id expected=%d given=%d", op1.id, s.id)
	}

	if ok := s.TryUnlock(op2); ok {
		t.Fatalf("should not have unlocked")
	} else if s.id != op1.id {
		t.Fatalf("wrong id expected=%d given=%d", op1.id, s.id)
	}

	if ok := s.TryUnlock(op1); !ok {
		t.Fatalf("should have unlocked")
	} else if s.id != 0 {
		t.Fatalf("wrong id expected=%d given=%d", 0, s.id)
	}
}

func TestSoftMutexLockPanic(t *testing.T) {
	valid := false

	defer func() {
		if !valid {
			t.Fatalf("should have panicked")
		}
	}()

	defer func() {
		if err := recover(); err != nil {
			valid = true
		}
	}()

	s := &softMutex{}

	s.Lock(op1)
	s.Lock(op1)
}

func TestSoftMutexTryLockPanic(t *testing.T) {
	valid := false

	defer func() {
		if !valid {
			t.Fatalf("should have panicked")
		}
	}()

	defer func() {
		if err := recover(); err != nil {
			valid = true
		}
	}()

	s := &softMutex{}

	s.TryLock(op1)
	s.TryLock(op1)
}

func TestSoftMutexUnlockPanic(t *testing.T) {
	valid := false

	defer func() {
		if !valid {
			t.Fatalf("should have panicked")
		}
	}()

	defer func() {
		if err := recover(); err != nil {
			valid = true
		}
	}()

	s := &softMutex{}

	s.Lock(op1)
	s.Unlock(op1)
	s.Unlock(op2)
}
