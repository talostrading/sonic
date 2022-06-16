package sonic

import (
	"runtime"
	"testing"
	"time"

	"github.com/talostrading/sonic/internal"
)

func TestPost(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	xs := []bool{false, false, false}

	for i := 0; i < len(xs); i++ {
		// We need to copy i otherwise xs[i] will panic because i == len(xs)
		// when the loop is done.
		j := i
		ioc.Post(func() {
			xs[j] = true
		})
	}

	if p := ioc.Pending(); p != 3 {
		t.Fatalf("not accounting for pending operations correctly expected=%d given=%d", 3, p)
	}

	ioc.RunPending()

	for i, x := range xs {
		if !x {
			t.Fatalf("handler %d not set", i)
		}
	}

	if p := ioc.Pending(); p != 0 {
		t.Fatalf("not accounting for pending operations correctly expected=%d given=%d", 0, p)
	}
}

func TestEmptyPoll(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	if err := ioc.Poll(); err != internal.ErrTimeout {
		t.Fatalf("expected timeout as not operations are scheduled")
	}
}

func TestRunOneFor(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	start := time.Now()

	expected := 5 * time.Millisecond
	if err := ioc.RunOneFor(expected); err != internal.ErrTimeout {
		t.Fatalf("expected timeout as no operations are scheduled received=%v", err)
	}

	end := time.Now()

	if given := end.Sub(start); given.Milliseconds() < expected.Milliseconds() {
		t.Fatalf("invalid timeout ioc.RunOneFor(...) expected=%v given=%v", expected, given)
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
