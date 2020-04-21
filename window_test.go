package circuit

import (
	"testing"
	"time"

	"github.com/facebookgo/clock"
)

func TestWindowCounts(t *testing.T) {
	w := newWindow(time.Millisecond*10, 2, clock.New(), 0)
	w.Fail()
	w.Fail()
	w.Success()
	w.Success()

	if f := w.Failures(); f != 2 {
		t.Fatalf("expected window to have 2 failures, got %d", f)
	}

	if s := w.Successes(); s != 2 {
		t.Fatalf("expected window to have 2 successes, got %d", s)
	}

	if r := w.ErrorRate(); r != 0.5 {
		t.Fatalf("expected window to have 0.5 error rate, got %f", r)
	}

	w.Reset()
	if f := w.Failures(); f != 0 {
		t.Fatalf("expected reset window to have 0 failures, got %d", f)
	}
	if s := w.Successes(); s != 0 {
		t.Fatalf("expected window to have 0 successes, got %d", s)
	}
}

func TestWindowSlides(t *testing.T) {
	c := clock.NewMock()

	w := newWindow(time.Millisecond*10, 2, c, 0)

	w.Fail()
	c.Add(time.Millisecond * 6)
	w.Fail()

	counts := 0
	for i := 0; i < len(w.buckets); i++ {
		b := &w.buckets[i]
		if b.failure > 0 {
			counts++
		}
	}

	if counts != 2 {
		t.Fatalf("expected 2 buckets to have failures, got %d", counts)
	}

	c.Add(time.Millisecond * 15)
	w.Success()
	counts = 0
	for i := 0; i < len(w.buckets); i++ {
		b := &w.buckets[i]
		if b.failure > 0 {
			counts++
		}
	}

	if counts != 0 {
		t.Fatalf("expected 0 buckets to have failures, got %d", counts)
	}
}

func TestWindowSmooth(t *testing.T) {
	c := clock.NewMock()
	limited := 256
	n := 5
	k := 10
	w := newWindow(time.Second*time.Duration(n), n, c, limited)

	for i := 0; i < limited*k*n; i++ {
		w.Fail()
		if i%limited == 0 {
			c.Add(time.Second / time.Duration(k))
		}
	}

	f := w.Failures()
	if l := limited * n; f < int64(l) {
		t.Fatalf("expected w.Failures() > %d, got %d", l, f)
	}
	if l := limited * k * n; f >= int64(l) {
		t.Fatalf("expected w.Failures() <= %d, got %d", l, f)
	}

	c.Add(time.Second * time.Duration(n) * 2)
	w.Success()
	c.Add(time.Second * time.Duration(n) * 2)
	w.Success()

	for i := 0; i < len(w.buckets); i++ {
		if l := w.buckets[i].limited; l != int64(limited) {
			t.Fatalf("expected w.buckets[%d].limited == %d, got %d",
				i, limited, l)
		}
	}
}
