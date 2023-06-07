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
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

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
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

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
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

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
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

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
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

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
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

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
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

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
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

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
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

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
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	done := false
	err = timer.ScheduleOnce(0, func() {
		done = true
	})
	if err != nil {
		t.Fatal(err)
	}

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
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

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

func TestTimerScheduleOnceConsecutively(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	var start, end time.Time
	start = time.Now()
	err = timer.ScheduleOnce(10*time.Millisecond, func() {
		err = timer.ScheduleOnce(10*time.Millisecond, func() {
			err = timer.ScheduleOnce(10*time.Millisecond, func() {
				end = time.Now()
			})
			if err != nil {
				t.Fatal(err)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	err = timer.ScheduleOnce(-1, func() {})
	if !errors.Is(err, sonicerrors.ErrCancelled) {
		t.Fatalf("should not be able to schedule an already scheduled clock, expected=%s", sonicerrors.ErrCancelled)
	}

	for i := 0; i < 3; i++ {
		ioc.RunOne()
	}
	dur := end.Sub(start)
	if dur <= 29*time.Millisecond { // 1ms error margin
		t.Fatal("slept for the wrong period of time")
	}
}

func TestTimerScheduleOnceConsecutivelySleepInBetween(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	var start, end time.Time
	start = time.Now()
	err = timer.ScheduleOnce(10*time.Millisecond, func() {
		time.Sleep(10 * time.Millisecond)
		err = timer.ScheduleOnce(10*time.Millisecond, func() {
			time.Sleep(10 * time.Millisecond)
			err = timer.ScheduleOnce(10*time.Millisecond, func() {
				time.Sleep(10 * time.Millisecond)
				end = time.Now()
			})
			if err != nil {
				t.Fatal(err)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	err = timer.ScheduleOnce(-1, func() {})
	if !errors.Is(err, sonicerrors.ErrCancelled) {
		t.Fatalf("should not be able to schedule an already scheduled clock, expected=%s", sonicerrors.ErrCancelled)
	}

	ioc.RunPending()
	dur := end.Sub(start)
	if dur <= 59*time.Millisecond { // 1ms error margin
		t.Fatalf("slept for the wrong period of time %s", dur)
	}
}

func TestTimerScheduleRepeatingConsecutively(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := timer.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	var start, end time.Time
	start = time.Now()
	err = timer.ScheduleRepeating(10*time.Millisecond, func() {
		err = timer.Cancel()
		if err != nil {
			t.Fatal(err)
		}

		err = timer.ScheduleRepeating(10*time.Millisecond, func() {
			err = timer.Cancel()
			if err != nil {
				t.Fatal(err)
			}

			err = timer.ScheduleRepeating(10*time.Millisecond, func() {
				err = timer.Cancel()
				if err != nil {
					t.Fatal(err)
				}

				end = time.Now()
			})
			if err != nil {
				t.Fatal(err)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	err = timer.ScheduleRepeating(-1, func() {})
	if !errors.Is(err, sonicerrors.ErrCancelled) {
		t.Fatalf("should not be able to schedule an already scheduled clock, expected=%s", sonicerrors.ErrCancelled)
	}

	for i := 0; i < 3; i++ {
		ioc.RunOne()
	}
	dur := end.Sub(start)
	if dur <= 29*time.Millisecond { // 1ms error margin
		t.Fatal("slept for the wrong period of time")
	}
}

func TestTimerScheduleRepeatingAndCancel(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		called := 0
		err := timer.ScheduleRepeating(time.Millisecond, func() {
			called++
			if called == 5 {
				timer.Cancel()
			}
		})
		if err != nil {
			t.Fatal(err)
		}

		for i := 0; i < 5; i++ {
			if !timer.Scheduled() {
				t.Fatal("timer should be scheduled")
			}
			if len(timer.ioc.pendingTimers) != 1 {
				t.Fatal("there should be a pending timer")
			}
			ioc.RunOne()
		}
		if timer.Scheduled() {
			t.Fatal("timer should not be scheduled")
		}
		if timer.cancelled {
			t.Fatal("timer.cancelled should not be false")
		}
		if timer.state != stateReady {
			t.Fatal("timer should be in ready state")
		}
		if len(timer.ioc.pendingTimers) != 0 {
			t.Fatal("there should be no pending timers")
		}
	}

	once := false
	err = timer.ScheduleOnce(time.Millisecond, func() {
		once = true
	})
	if err != nil {
		t.Fatal(err)
	}
	ioc.RunOne()
	if len(timer.ioc.pendingTimers) != 0 {
		t.Fatal("there should be no pending timers")
	}
	if !once {
		t.Fatal("schedule once did not call")
	}
	if timer.Scheduled() {
		t.Fatal("timer should not be scheduled")
	}
	if timer.state != stateReady {
		t.Fatal("timer should be in ready state")
	}
}

func TestTimerCancelAndThenScheduleRepeating(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	// here we are cancelling the timer before we call ScheduleRepeating to make sure
	// that ScheduleRepeating will set cancelled to false and will do all the iterations as it should
	timer.Cancel()

	// Scheduling a timer & cancelling it after 5 iterations
	called := 0
	err = timer.ScheduleRepeating(time.Millisecond, func() {
		called++
		if called == 5 {
			timer.Cancel()
		}
	})

	if err != nil {
		t.Fatal(err)
	}

	if !timer.Scheduled() {
		t.Fatal("timer should be scheduled")
	}
	if timer.state != stateScheduled {
		t.Fatal("timer.state should be scheduled")
	}
	if timer.cancelled {
		t.Fatal("timer.cancelled should be false")
	}

	// run 5 jobs to get the 5 callbacks triggered
	for i := 0; i < 5; i++ {
		ioc.RunOne()
	}

	if timer.Scheduled() {
		t.Fatal("timer should not be scheduled")
	}
	if timer.cancelled {
		t.Fatal("timer.cancelled should be false")
	}
	if timer.state != stateReady {
		t.Fatal("timer should be in ready state")
	}
	if called != 5 {
		t.Fatal("timer didn't trigger 5 times as it should have")
	}
}

func BenchmarkTimerNew(b *testing.B) {
	ioc := MustIO()
	defer ioc.Close()

	// just to see the allocations
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t, err := NewTimer(ioc)
		if err != nil {
			panic(err)
		}
		t.Close()
	}
	b.ReportAllocs()
}

func BenchmarkTimerScheduleOnce(b *testing.B) {
	ioc := MustIO()
	defer ioc.Close()

	t, err := NewTimer(ioc)
	if err != nil {
		panic(err)
	}
	defer t.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t.ScheduleOnce(time.Nanosecond, func() {})
	}
	b.ReportAllocs()
}

func BenchmarkTimerScheduleRepeating(b *testing.B) {
	ioc := MustIO()
	defer ioc.Close()

	t, err := NewTimer(ioc)
	if err != nil {
		panic(err)
	}
	defer t.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t.ScheduleRepeating(time.Nanosecond, func() {})
	}
	b.ReportAllocs()
}
