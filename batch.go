// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
)

// skipped is the error a call of forEachFile returns for a file that was left
// alone on purpose.  The reason is written, but the call does not count as a
// failure.
type skipped struct {
	reason string
}

func (s skipped) Error() string {
	return s.reason
}

// forEachFile calls do for each of the files, with at most jobs calls running
// at once.  Each call gets a logger of its own.  What a call logged, then its
// error if it failed, is written to w in the order of files, as soon as every
// earlier file has finished.  A call that returns skipped is not a failure.  It
// reports whether every call succeeded.
func forEachFile(files []string, jobs int, w io.Writer, do func(file string, logger *log.Logger) error) bool {
	type result struct {
		logged bytes.Buffer
		err    error
		done   chan struct{}
	}

	results := make([]*result, len(files))
	for i := range results {
		results[i] = &result{done: make(chan struct{})}
	}

	// A separate goroutine starts the calls, so results can be written while
	// it waits for a free slot.
	go func() {
		slots := make(chan struct{}, max(jobs, 1))
		for i, file := range files {
			slots <- struct{}{}
			go func() {
				defer func() { <-slots }()

				r := results[i]
				defer close(r.done)

				r.err = do(file, log.New(&r.logged, "", log.LstdFlags))
			}()
		}
	}()

	ok := true
	for i, file := range files {
		r := results[i]
		<-r.done

		_, _ = w.Write(r.logged.Bytes())

		var skip skipped
		switch {
		case r.err == nil:
		case errors.As(r.err, &skip):
			_, _ = fmt.Fprintf(w, "%s: %s: %s\n", name, file, skip.reason)
		default:
			ok = false
			_, _ = fmt.Fprintf(w, "%s: error: %s: %v\n", name, file, r.err)
		}
	}

	return ok
}
