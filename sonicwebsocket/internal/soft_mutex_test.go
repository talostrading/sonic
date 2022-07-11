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

var op1 = OpRead
var op2 = OpHandshake

func TestMutex(t *testing.T) {
	s := &SoftMutex{}

	s.Lock(op1)
	if s.op != OpRead {
		t.Fatalf("wrong id expected=%s given=%s", op1, s.op)
	}

	s.Unlock(op1)
	if s.op != OpNoop {
		t.Fatalf("wrong id expected=%s given=%s", OpNoop, s.op)
	}

	if ok := s.TryLock(op1); !ok {
		t.Fatalf("should have locked")
	} else if s.op != op1 {
		t.Fatalf("wrong id expected=%s given=%s", op1, s.op)
	}

	if ok := s.TryLock(op2); ok {
		t.Fatalf("should not have locked")
	} else if s.op != op1 {
		t.Fatalf("wrong id expected=%s given=%s", op1, s.op)
	}

	if ok := s.TryUnlock(op2); ok {
		t.Fatalf("should not have unlocked")
	} else if s.op != op1 {
		t.Fatalf("wrong id expected=%s given=%s", op1, s.op)
	}

	if ok := s.TryUnlock(op1); !ok {
		t.Fatalf("should have unlocked")
	} else if s.op != OpNoop {
		t.Fatalf("wrong id expected=%s given=%s", OpNoop, s.op)
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

	s := &SoftMutex{}

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

	s := &SoftMutex{}

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

	s := &SoftMutex{}

	s.Lock(op1)
	s.Unlock(op1)
	s.Unlock(op2)
}
