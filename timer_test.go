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

func TestTimerZeroTimeout(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	done := false
	err = timer.ScheduleOnce(0, func() {
		done = true
	})

	ioc.PollOne()

	if !done {
		t.Fatal("timer did not fire")
	}
}

func TestTimerCancel(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	// schedule once and cancel before trigger
	triggered := false
	err = timer.ScheduleOnce(5*time.Millisecond, func() { triggered = true })
	if err != nil {
		t.Fatal(err)
	}

	_, err = ioc.PollOne()
	if !errors.Is(err, sonicerrors.ErrTimeout) {
		t.Fatal(err)
	}

	if timer.state != stateScheduled {
		t.Fatal("timer should be in scheduled state")
	}

	if !timer.Scheduled() {
		t.Fatal("timer should be scheduled")
	}

	if ioc.Pending() != 1 {
		t.Fatal("ioc should have one pending timer")
	}

	// cancel and make sure it does not trigger
	err = timer.Cancel()
	if err != nil {
		t.Fatal(err)
	}

	if ioc.Pending() != 0 {
		t.Fatal("ioc should have no pending operations")
	}

	if timer.Scheduled() {
		t.Fatal("timer should not be scheduled")
	}

	err = ioc.RunOneFor(10 * time.Millisecond)
	if !errors.Is(err, sonicerrors.ErrTimeout) {
		t.Fatal(err)
	}

	if triggered {
		t.Fatal("cancelled timer should have not triggered")
	}

	if timer.state != stateReady {
		t.Fatal("time should be in ready state")
	}

	// schedule again and let it trigger
	triggered = false
	err = timer.ScheduleOnce(10*time.Millisecond, func() {
		triggered = true
	})
	if err != nil {
		t.Fatal(err)
	}

	err = ioc.RunOneFor(5 * time.Millisecond)
	if !errors.Is(err, sonicerrors.ErrTimeout) {
		t.Fatal(err)
	}

	if triggered {
		t.Fatal("timer should not have triggered")
	}

	err = ioc.RunOneFor(20 * time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	if !triggered {
		t.Fatal("timer should have triggered")
	}
}
