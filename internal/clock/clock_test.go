package clock

import (
	"testing"
	"time"
)

func TestFakeNowAdvance(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(start)
	if got := f.Now(); !got.Equal(start) {
		t.Errorf("Now = %v, want %v", got, start)
	}
	f.Advance(time.Hour)
	if got := f.Now(); !got.Equal(start.Add(time.Hour)) {
		t.Errorf("after Advance: Now = %v", got)
	}
}

func TestFakeAfter(t *testing.T) {
	f := NewFake(time.Now())
	ch := f.After(500 * time.Millisecond)

	select {
	case <-ch:
		t.Fatal("After channel fired before Advance")
	default:
	}

	f.Advance(400 * time.Millisecond)
	select {
	case <-ch:
		t.Fatal("After channel fired prematurely")
	default:
	}

	f.Advance(200 * time.Millisecond)
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("After channel did not fire after Advance past deadline")
	}
}

func TestFakeAfterMultipleWaiters(t *testing.T) {
	f := NewFake(time.Now())
	a := f.After(1 * time.Second)
	b := f.After(2 * time.Second)

	f.Advance(1500 * time.Millisecond)
	select {
	case <-a:
	case <-time.After(time.Second):
		t.Fatal("first waiter did not fire")
	}
	select {
	case <-b:
		t.Fatal("second waiter fired too early")
	default:
	}

	f.Advance(time.Second)
	select {
	case <-b:
	case <-time.After(time.Second):
		t.Fatal("second waiter did not fire")
	}
}
