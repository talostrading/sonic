package internal

import "fmt"

type softMutex struct {
	id int
}

func (s *softMutex) Reset() {
	s.id = 0
}

func (s *softMutex) IsLocked() bool {
	return s.id != 0
}

func (s *softMutex) IsLockedWith(op operation) bool {
	return s.id == op.ID()
}

func (s *softMutex) Lock(op operation) {
	if s.id != 0 {
		panic(fmt.Errorf("attempting to lock operation=%d while already locked on op=%d", op.ID(), s.id))
	}
	s.id = op.ID()
}

func (s *softMutex) Unlock(op operation) {
	if s.id != op.ID() {
		panic(fmt.Errorf("attempting to unlock operation=%d while locked on a different op=%d", op.ID(), s.id))
	}
	s.id = 0
}

func (s *softMutex) TryLock(op operation) bool {
	if s.id == op.ID() {
		panic(fmt.Errorf("trying to lock an already locked op=%d", op.ID()))
	}
	if s.id != 0 {
		return false
	}
	s.id = op.ID()
	return true
}

func (s *softMutex) TryUnlock(op operation) bool {
	if s.id != op.ID() {
		return false
	}
	s.id = 0
	return true
}
