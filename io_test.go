package sonic

import (
	"errors"
	"log"
	"runtime"
	"testing"
	"time"

	"github.com/talostrading/sonic/internal"
	"github.com/talostrading/sonic/sonicerrors"
)

func TestPost(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	var xs [1000]bool

	for i := 0; i < len(xs); i++ {
		// We need to copy i otherwise xs[i] will panic because i == len(xs)
		// when the loop is done.
		j := i
		ioc.Post(func() {
			xs[j] = true
		})
	}

	if ioc.Posted() != 1000 {
		t.Fatal("expected 1000 posted events")
	}

	if p := ioc.Pending(); p != 1000 {
		t.Fatalf("not accounting for pending operations correctly expected=%d given=%d", 1000, p)
	}

	err := ioc.RunPending()
	if err != nil {
		t.Fatal(err)
	}

	for i, x := range xs {
		if !x {
			t.Fatalf("handler %d not set", i)
		}
	}

	if p := ioc.Pending(); p != 0 {
		t.Fatalf("not accounting for pending operations correctly expected=%d given=%d", 0, p)
	}

	if g := ioc.Posted(); g != 0 {
		t.Fatalf("expected 0 posted events but got %d", g)
	}
}

func TestEmptyPoll(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	if err := ioc.Poll(); !errors.Is(err, sonicerrors.ErrTimeout) {
		t.Fatalf("expected timeout as not operations are scheduled")
	}
}

func TestRunOneFor(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	start := time.Now()

	expected := time.Millisecond
	if err := ioc.RunOneFor(expected); !errors.Is(err, sonicerrors.ErrTimeout) {
		t.Fatalf(
			"expected timeout as no operations are scheduled received=%v",
			err)
	}

	end := time.Now()

	if given := end.Sub(start); given.Milliseconds() < expected.Milliseconds() {
		t.Fatalf(
			"invalid timeout ioc.RunOneFor(...) expected=%v given=%v",
			expected,
			given,
		)
	}
}

func TestRightNumberOfPolledEvents(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}

	dur := 500 * time.Millisecond
	err = timer.ScheduleOnce(dur, func() {})
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	npolled := 0
	for {
		if time.Now().Sub(start) > 2*dur {
			break
		}

		n, err := ioc.PollOne()
		if err != nil && !errors.Is(err, sonicerrors.ErrTimeout) {
			t.Fatal(err)
		}

		if n > npolled {
			npolled = n
		}
	}

	if npolled != 1 {
		t.Fatalf("expected to poll 1 operation, but polled %d", npolled)
	}
}

func TestPollOneAfterClose(t *testing.T) {
	ioc := MustIO()

	if ioc.Closed() {
		t.Fatal("ioc should not be closed")
	}

	n, err := ioc.PollOne()
	if err != nil && !errors.Is(err, sonicerrors.ErrTimeout) {
		t.Fatal(err)
	}

	if n != 0 {
		t.Fatalf("polled %d but should not have polled any", n)
	}

	err = ioc.Close()
	if err != nil {
		t.Fatal(err)
	}

	if !ioc.Closed() {
		t.Fatal("ioc should be closed")
	}

	n, err = ioc.PollOne()
	if err == nil || errors.Is(err, sonicerrors.ErrTimeout) || n != 0 {
		t.Fatalf("the poll should have failed after close n=%d err=%v", n, err)
	}
}

func TestRunOneForAfterClose(t *testing.T) {
	ioc := MustIO()

	if ioc.Closed() {
		t.Fatal("ioc should not be closed")
	}

	err := ioc.RunOneFor(time.Millisecond)
	if err != nil && !errors.Is(err, sonicerrors.ErrTimeout) {
		t.Fatal(err)
	}

	err = ioc.Close()
	if err != nil {
		t.Fatal(err)
	}

	if !ioc.Closed() {
		t.Fatal("ioc should be closed")
	}

	err = ioc.RunOneFor(time.Millisecond)
	if err == nil || errors.Is(err, sonicerrors.ErrTimeout) {
		t.Fatalf("the poll should have failed after close err=%v", err)
	}
}

func TestPollNothing(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	n, err := ioc.PollOne()
	if n != 0 && !errors.Is(err, sonicerrors.ErrTimeout) {
		t.Fatalf("wrong n=%d and err=%v", n, err)
	}
}

func TestSleep(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	timer, err := NewTimer(ioc)
	if err != nil {
		panic(err)
	}

	scheduled := time.Now()
	err = timer.ScheduleOnce(10*time.Millisecond, func() {
		time.Sleep(10 * time.Millisecond)
		err = timer.ScheduleOnce(10*time.Millisecond, func() {
			time.Sleep(10 * time.Millisecond)
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	ioc.RunPending()

	dur := time.Since(scheduled)
	log.Printf("slept for a total of %s, went over by %s", dur, dur-time.Duration(40)*time.Millisecond)
}

func TestRunWarmInvalidBusyCycles(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	if err := ioc.RunWarm(0, 2*time.Millisecond); err == nil {
		t.Fatal("should have errored: invalid busy-cycles")
	}
}

func TestRunWarmInvalidTimeout(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	if err := ioc.RunWarm(10, time.Microsecond); err == nil {
		t.Fatal("should have errored: invalid timeout")
	}
}

func TestRunWarm(t *testing.T) {
	// This test will always pass until we have some mechanism to measure the reactor's cycle duration.
	// TODO revisit after introducing CycleDur() in reactor.
	ioc := MustIO()
	defer ioc.Close()

	ticker, err := NewTimer(ioc)
	if err != nil {
		t.Fatal(err)
	}
	defer ticker.Close()

	i := 0
	ticker.ScheduleRepeating(time.Millisecond, func() {
		i++
		if i >= 10 {
			ioc.Close()
		}
	})

	// The ticker triggers every 1ms for 10 times, so we should run through the whole warm period and then yield
	// until the ticker triggers, for 10 times. Each yield be resumed by the ticker.
	if err := ioc.RunWarm(10, 2*time.Millisecond); err != nil {
		if i < 10 {
			// something happened before `Close()`ing the reactor.
			t.Fatal(err)
		}
	}
}

func TestResizePending(t *testing.T) {
	ioc := MustIO()

	called := 0
	handler := func(error) {
		called++
	}

	var slots []*internal.PollData
	for i := 0; i < 2*DefaultPendingCount; i++ {
		slots = append(slots, &internal.PollData{Fd: i})
		slots[len(slots)-1].Cbs[internal.ReadEvent] = handler
	}

	for i := 0; i < DefaultPendingCount; i++ {
		ioc.RegisterRead(slots[i])
		ioc.RegisterWrite(slots[i])
	}
	if len(ioc.pendingReads) != DefaultPendingCount {
		t.Fatal("pending reads should not have been resized")
	}
	if len(ioc.pendingWrites) != DefaultPendingCount {
		t.Fatal("pending writes should not have been resized")
	}

	for i := DefaultPendingCount; i < 2*DefaultPendingCount; i++ {
		ioc.RegisterRead(slots[i])
		ioc.RegisterWrite(slots[i])
	}
	if len(ioc.pendingReads) < 2*DefaultPendingCount {
		t.Fatal("pending reads should have been resized")
	}
	if len(ioc.pendingWrites) < 2*DefaultPendingCount {
		t.Fatal("pending writes should have been resized")
	}

	for i := 0; i < 2*DefaultPendingCount; i++ {
		ioc.pendingReads[i].Cbs[internal.ReadEvent](nil)
		ioc.pendingWrites[i].Cbs[internal.ReadEvent](nil)
	}

	if called < 2*DefaultPendingCount*2 /* read + write */ {
		t.Fatal("handler not called enough times")
	}
}

func BenchmarkPollOne(b *testing.B) {
	ioc := MustIO()
	defer ioc.Close()

	runtime.GOMAXPROCS(1)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for i := 0; i < b.N; i++ {
		ioc.PollOne()
	}
}
