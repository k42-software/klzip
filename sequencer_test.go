// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

// within fails the test if f does not return within a few seconds.
func within(t *testing.T, what string, f func()) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not finish: a position is waiting forever", what)
	}
}

// Positions that start in the wrong order still write in the order of their
// positions.
func TestSequencerWritesInOrder(t *testing.T) {
	var out bytes.Buffer
	s := newSequencer(&out, 3)

	var wg sync.WaitGroup
	for _, n := range []int{2, 1, 0} {
		wg.Add(1)
		go func() {
			defer wg.Done()

			w := s.writer(n)
			defer w.Done()

			// The earlier positions are slower to start.
			time.Sleep(time.Duration(2-n) * 40 * time.Millisecond)
			_, _ = w.Write([]byte{byte('0' + n)})
		}()
	}

	within(t, "writing", wg.Wait)

	if got, want := out.String(), "012"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// A write blocks until every earlier position is done.
func TestSequencerWriteWaitsForEarlierPositions(t *testing.T) {
	var out bytes.Buffer
	s := newSequencer(&out, 2)

	written := make(chan struct{})
	go func() {
		defer close(written)
		_, _ = s.writer(1).Write([]byte("late"))
	}()

	select {
	case <-written:
		t.Fatal("position 1 wrote before position 0 was done")
	case <-time.After(100 * time.Millisecond):
	}

	s.writer(0).Done()

	within(t, "the write", func() { <-written })

	if got, want := out.String(), "late"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// A position that is done without writing anything lets the next one write.
func TestSequencerDoneWithoutWriting(t *testing.T) {
	var out bytes.Buffer
	s := newSequencer(&out, 2)

	s.writer(0).Done()

	within(t, "the write", func() {
		_, _ = s.writer(1).Write([]byte("next"))
	})

	if got, want := out.String(), "next"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// Positions can be done in any order.  When the earliest one is done, every
// position after it that was already done is skipped over.
func TestSequencerDoneOutOfOrder(t *testing.T) {
	var out bytes.Buffer
	s := newSequencer(&out, 4)

	s.writer(2).Done()
	s.writer(1).Done()

	// Position 0 is not done, so position 3 must still wait.
	blocked := make(chan struct{})
	go func() {
		defer close(blocked)
		_, _ = s.writer(3).Write([]byte("three"))
	}()
	select {
	case <-blocked:
		t.Fatal("position 3 wrote before position 0 was done")
	case <-time.After(100 * time.Millisecond):
	}

	s.writer(0).Done()

	within(t, "position 3", func() { <-blocked })

	if got, want := out.String(), "three"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestSequencerDoneTwice(t *testing.T) {
	var out bytes.Buffer
	s := newSequencer(&out, 2)

	w := s.writer(0)
	w.Done()
	w.Done()

	within(t, "the write", func() {
		_, _ = s.writer(1).Write([]byte("ok"))
	})
}
