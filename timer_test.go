package sonic

import (
	"testing"
	"time"

	"github.com/talostrading/sonic/sonicerrors"
)

func TestTimerScheduleOnce(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	if timer.Scheduled() {
		t.Fatal("timer should not be scheduled")
	}

	dur := time.Millisecond

	done := false
	err = timer.ScheduleOnce(dur, func() {
		done = true
	})
	if err != nil {
		t.Fatal(err)
	}

	f := time.NewTimer(5 * dur)

outer:
	for {
		select {
		case <-f.C:
			break outer
		default:
		}

		ioc.PollOne()
	}

	if !done {
		t.Fatal("timer did not fire")
	}

	if timer.state != stateReady {
		t.Fatal("timer should be ready")
	}
}

func TestTimerScheduleOnceAndClose(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	if timer.Scheduled() {
		t.Fatal("timer should not be scheduled")
	}

	dur := time.Millisecond

	done := false
	err = timer.ScheduleOnce(dur, func() {
		done = true
		timer.Close()
	})
	if err != nil {
		t.Fatal(err)
	}

	f := time.NewTimer(5 * dur)

outer:
	for {
		select {
		case <-f.C:
			break outer
		default:
		}

		ioc.PollOne()
	}

	if !done {
		t.Fatal("timer did not fire")
	}

	if timer.state != stateClosed {
		t.Fatal("timer should be closed")
	}

	err = timer.ScheduleOnce(time.Second, func() {})
	if err != sonicerrors.ErrCancelled {
		t.Fatal("operation should be cancelled")
	}
}

func TestTimerScheduleRepeating(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	if timer.Scheduled() {
		t.Fatal("timer should not be scheduled")
	}

	dur := time.Millisecond

	fired := 0
	err = timer.ScheduleRepeating(dur, func() {
		if timer.Scheduled() {
			t.Fatal("timer should not be scheduled")
		}
		fired++
	})
	if err != nil {
		t.Fatal(err)
	}

	f := time.NewTimer(10 * dur)

outer:
	for {
		select {
		case <-f.C:
			break outer
		default:
		}

		ioc.PollOne()
	}

	if fired <= 5 {
		t.Fatal("timer did not fire repeteadly")
	}

	if timer.state != stateScheduled {
		t.Fatalf("timer should be ready is=%v", timer.state)
	}
}

func TestTimerScheduleRepeatingAndClose(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	if timer.Scheduled() {
		t.Fatal("timer should not be scheduled")
	}

	dur := time.Millisecond

	fired := 0
	err = timer.ScheduleRepeating(dur, func() {
		if timer.Scheduled() {
			t.Fatal("timer should not be scheduled")
		}
		fired++
		if fired == 5 {
			timer.Close()
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	f := time.NewTimer(10 * dur)

outer:
	for {
		select {
		case <-f.C:
			break outer
		default:
		}

		ioc.PollOne()
	}

	if timer.state != stateClosed {
		t.Fatal("timer should be closed")
	}

	err = timer.ScheduleOnce(time.Second, func() {})
	if err != sonicerrors.ErrCancelled {
		t.Fatal("operation should be cancelled")
	}
}
