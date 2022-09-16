package sonic

import (
	"errors"
	"testing"
	"time"

	"github.com/talostrading/sonic/sonicerrors"
)

const TimerTestDuration = time.Millisecond

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

	done := false
	err = timer.ScheduleOnce(TimerTestDuration, func() {
		done = true
	})
	if err != nil {
		t.Fatal(err)
	}

	if timer.state != stateScheduled {
		t.Fatal("timer should be in stateScheduled")
	}

	f := time.NewTimer(2 * TimerTestDuration)

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
		t.Fatal("timer should be in stateReady")
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

	done := false
	err = timer.ScheduleOnce(TimerTestDuration, func() {
		done = true
		timer.Close()
	})
	if err != nil {
		t.Fatal(err)
	}

	if timer.state != stateScheduled {
		t.Fatal("timer should be in stateScheduled")
	}

	f := time.NewTimer(2 * TimerTestDuration)

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
		t.Fatal("timer should be stateClosed")
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

	fired := 0
	err = timer.ScheduleRepeating(TimerTestDuration, func() {
		if timer.Scheduled() {
			t.Fatal("timer should not be scheduled")
		}
		fired++
	})
	if err != nil {
		t.Fatal(err)
	}

	if timer.state != stateScheduled {
		t.Fatal("timer should be in stateScheduled")
	}

	f := time.NewTimer(5 * TimerTestDuration)

outer:
	for {
		select {
		case <-f.C:
			break outer
		default:
		}

		ioc.PollOne()
	}

	if fired <= 1 {
		t.Fatal("timer did not fire repeteadly")
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

	fired := 0
	err = timer.ScheduleRepeating(TimerTestDuration, func() {
		if timer.Scheduled() {
			t.Fatal("timer should not be scheduled")
		}
		fired++
		if fired >= 2 {
			timer.Close()
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	if timer.state != stateScheduled {
		t.Fatal("timer should be in stateScheduled")
	}

	f := time.NewTimer(5 * TimerTestDuration)

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
		t.Fatal("timer should be in stateClosed")
	}
}

func TestTimerCloseAfterScheduleOnce(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	shouldNotBeTrue := false

	err = timer.ScheduleOnce(TimerTestDuration, func() {
		shouldNotBeTrue = true
	})
	if err != nil {
		t.Fatal(err)
	}

	err = timer.Close()
	if err != nil {
		t.Fatal(err)
	}

	if timer.state != stateClosed {
		t.Fatal("timer should be in stateClosed")
	}

	f := time.NewTimer(2 * TimerTestDuration)
outer:
	for {
		select {
		case <-f.C:
			break outer
		default:
		}
		ioc.PollOne()
	}

	if shouldNotBeTrue {
		t.Fatal("callback triggered after closing the timer")
	}
}

func TestTimerCloseAfterScheduleRepeating(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	shouldNotBeTrue := false

	err = timer.ScheduleRepeating(TimerTestDuration, func() {
		shouldNotBeTrue = true
	})
	if err != nil {
		t.Fatal(err)
	}

	err = timer.Close()
	if err != nil {
		t.Fatal(err)
	}

	if timer.state != stateClosed {
		t.Fatal("timer should be in stateClosed")
	}

	f := time.NewTimer(2 * TimerTestDuration)
outer:
	for {
		select {
		case <-f.C:
			break outer
		default:
		}
		ioc.PollOne()
	}

	if shouldNotBeTrue {
		t.Fatal("callback triggered after closing the timer")
	}
}

func TestTimerScheduleOnceWhileAlreadyScheduled(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	err = timer.ScheduleOnce(TimerTestDuration, func() {})
	if err != nil {
		t.Fatal(err)
	}

	err = timer.ScheduleOnce(TimerTestDuration, func() {})
	if !errors.Is(err, sonicerrors.ErrCancelled) {
		t.Fatal("operation should have been cancelled")
	}
}

func TestTimerScheduleRepeatingWhileAlreadyScheduled(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	err = timer.ScheduleRepeating(TimerTestDuration, func() {})
	if err != nil {
		t.Fatal(err)
	}

	err = timer.ScheduleOnce(TimerTestDuration, func() {})
	if !errors.Is(err, sonicerrors.ErrCancelled) {
		t.Fatal("operation should have been cancelled")
	}
}

func TestTimerCloseAfterClose(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		err = timer.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
