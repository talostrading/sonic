package internal

import "fmt"

type SoftMutex struct {
	op Operation
}

func (s *SoftMutex) Reset() {
	s.op = OpNoop
}

func (s *SoftMutex) IsLocked() bool {
	return s.op != OpNoop
}

func (s *SoftMutex) IsLockedWith(op Operation) bool {
	return s.op == op
}

func (s *SoftMutex) Lock(op Operation) {
	if s.op != OpNoop {
		panic(fmt.Errorf("attempting to lock operation=%d while already locked on op=%d", op, s.op))
	}
	s.op = op
}

func (s *SoftMutex) Unlock(op Operation) {
	if s.op != op {
		panic(fmt.Errorf("attempting to unlock operation=%s while locked on a different op=%s", op, s.op))
	}
	s.op = OpNoop
}

func (s *SoftMutex) TryLock(op Operation) bool {
	if s.op == op {
		panic(fmt.Errorf("trying to lock an already locked op=%s", op))
	}
	if s.op != OpNoop {
		return false
	}
	s.op = op
	return true
}

func (s *SoftMutex) TryUnlock(op Operation) bool {
	if s.op != op {
		return false
	}
	s.op = OpNoop
	return true
}
