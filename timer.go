package sonic

import (
	"time"

	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
)

type timerState uint8

const (
	stateReady timerState = iota
	stateScheduled
	stateClosed
)

func (s timerState) String() string {
	switch s {
	case stateReady:
		return "state_ready"
	case stateScheduled:
		return "state_scheduled"
	case stateClosed:
		return "state_closed"
	default:
		return "unknown_state"
	}
}

type Timer struct {
	ioc   *IO
	it    *internal.Timer
	state timerState

	// This is only checked in ScheduleRepeating. It is set in Cancel.
	// This ensures that we do not schedule the timer again if the ScheduleRepeating
	// callback cancelled the timer.
	cancelled bool
}

func NewTimer(ioc *IO) (*Timer, error) {
	it, err := internal.NewTimer(ioc.poller)
	if err != nil {
		return nil, err
	}

	return &Timer{
		ioc:   ioc,
		it:    it,
		state: stateReady,
	}, nil
}

// ScheduleOnce schedules a callback for execution after a delay.
//
// The callback is guaranteed to never be called before the delay.
// However, it is possible that it will be called a little after the delay.
//
// If the delay is negative or 0, the callback is executed as soon as possible.
func (t *Timer) ScheduleOnce(delay time.Duration, cb func()) (err error) {
	if t.state == stateReady {
		t.cancelled = false
		if delay <= 0 {
			cb()
		} else {
			err = t.it.Set(delay, func() {
				delete(t.ioc.pendingTimers, t)
				t.state = stateReady
				cb()
			})

			if err == nil {
				t.ioc.pendingTimers[t] = struct{}{}
				t.state = stateScheduled
			}
		}
	} else {
		err = sonicerrors.ErrCancelled
	}
	return
}

// ScheduleRepeating schedules a callback for execution once per interval.
//
// The callback is guaranteed to never be called before the repeat delay.
// However, it is possible that it will be called a little after the
// repeat delay.
//
// If the delay is negative or 0, the operation is cancelled.
func (t *Timer) ScheduleRepeating(repeat time.Duration, cb func()) error {
	if repeat <= 0 {
		return sonicerrors.ErrCancelled
	} else {
		var ccb func()
		ccb = func() {
			cb()
			if t.cancelled {
				t.cancelled = false
			} else {
				// TODO this error should not be ignored
				_ = t.ScheduleOnce(repeat, ccb)
			}
		}

		return t.ScheduleOnce(repeat, ccb)
	}
}

func (t *Timer) Scheduled() bool {
	return t.state == stateScheduled
}

func (t *Timer) Cancel() error {
	err := t.it.Unset()
	if err == nil {
		t.cancelled = true
		t.state = stateReady
	}
	return err
}

// Close closes the timer, render it useless for scheduling any more operations
// on it. A timer cannot be used after Close(). Any pending operations
// that have been scheduled but not yet completed are cancelled, and will
// therefore never complete.
func (t *Timer) Close() (err error) {
	if t.state != stateClosed {
		err = t.it.Close()
		if err == nil {
			t.state = stateClosed
			delete(t.ioc.pendingTimers, t)
		}
	}
	return
}
