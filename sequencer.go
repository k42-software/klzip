// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"sync"
)

// sequencer lets several goroutines share one writer by taking turns.  Each
// takes the position it will have in the output, counting from 0.  A position
// may write once every earlier position is done, so the output is in the order
// of the positions.  Until then its writes block, which holds the goroutine
// back.
type sequencer struct {
	w io.Writer

	mu   sync.Mutex
	turn *sync.Cond

	// done[n] is whether position n is done.
	done []bool

	// next is the lowest position that is not done.
	next int
}

// newSequencer returns a sequencer for the given number of positions.
func newSequencer(w io.Writer, positions int) *sequencer {
	s := &sequencer{w: w, done: make([]bool, positions)}
	s.turn = sync.NewCond(&s.mu)
	return s
}

// writer returns the writer for position n.
func (s *sequencer) writer(n int) *turnWriter {
	return &turnWriter{s: s, n: n}
}

// wait blocks until it is the turn of position n.
func (s *sequencer) wait(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for s.next < n {
		s.turn.Wait()
	}
}

// finish marks position n as done, so that the positions after it can take
// their turn.  Finishing a position more than once does no harm.
func (s *sequencer) finish(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.done[n] = true
	for s.next < len(s.done) && s.done[s.next] {
		s.next++
	}
	s.turn.Broadcast()
}

// turnWriter is the writer of one position of a sequencer.
type turnWriter struct {
	s *sequencer
	n int
}

// Write writes p to the shared writer, once it is the turn of this position.
// It must not be called after Done.
func (t *turnWriter) Write(p []byte) (int, error) {
	t.s.wait(t.n)
	return t.s.w.Write(p)
}

// Done gives up the turn of this position.  It must be called on every path
// out of the work of the position, including failures, or the positions after
// it wait forever.
func (t *turnWriter) Done() {
	t.s.finish(t.n)
}
