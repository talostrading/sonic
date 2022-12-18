package sonic

import (
	"errors"
	"log"
	"runtime"
	"testing"
	"time"

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
