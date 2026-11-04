// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Output is written in the order of the files, even when the first file is the
// last to finish.
func TestForEachFileWritesInOrder(t *testing.T) {
	delays := map[string]time.Duration{"a": 60 * time.Millisecond, "b": 30 * time.Millisecond, "c": 0}

	var out bytes.Buffer
	ok := forEachFile([]string{"a", "b", "c"}, 3, &out, func(file string, logger *log.Logger) error {
		time.Sleep(delays[file])
		logger.SetFlags(0)
		logger.Println("logged " + file)
		if file == "b" {
			return errors.New("it broke")
		}
		return nil
	})

	if ok {
		t.Error("ok = true, want false")
	}
	want := "logged a\nlogged b\nklzip: error: b: it broke\nlogged c\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// A file that fails does not stop the others from being done.
func TestForEachFileFailureDoesNotStopOthers(t *testing.T) {
	var done atomic.Int32

	var out bytes.Buffer
	ok := forEachFile([]string{"a", "b", "c", "d"}, 1, &out, func(file string, _ *log.Logger) error {
		done.Add(1)
		if file == "a" {
			return errors.New("it broke")
		}
		return nil
	})

	if ok {
		t.Error("ok = true, want false")
	}
	if got := done.Load(); got != 4 {
		t.Errorf("%d files were done, want 4", got)
	}
}

// A skipped file is reported but is not a failure, and the others are still
// done.
func TestForEachFileSkippedIsNotFailure(t *testing.T) {
	var done atomic.Int32

	var out bytes.Buffer
	ok := forEachFile([]string{"a", "b", "c"}, 1, &out, func(file string, _ *log.Logger) error {
		done.Add(1)
		if file == "b" {
			return skipped{reason: "left alone"}
		}
		return nil
	})

	if !ok {
		t.Error("ok = false, want true")
	}
	if got := done.Load(); got != 3 {
		t.Errorf("%d files were done, want 3", got)
	}
	if want := "klzip: b: left alone\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

func TestForEachFileSucceeds(t *testing.T) {
	var out bytes.Buffer
	ok := forEachFile([]string{"a", "b"}, 2, &out, func(string, *log.Logger) error { return nil })

	if !ok {
		t.Error("ok = false, want true")
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want none", out.String())
	}
}

func TestForEachFileNoFiles(t *testing.T) {
	var out bytes.Buffer
	if !forEachFile(nil, 2, &out, func(string, *log.Logger) error { return errors.New("called") }) {
		t.Error("ok = false, want true")
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want none", out.String())
	}
}

// With three jobs, three files are in progress at the same time.  Each call
// waits for the others, so a sequential run would never finish.
func TestForEachFileRunsInParallel(t *testing.T) {
	const jobs = 3

	var arrived sync.WaitGroup
	arrived.Add(jobs)

	allArrived := make(chan struct{})
	go func() {
		arrived.Wait()
		close(allArrived)
	}()

	var out bytes.Buffer
	ok := forEachFile([]string{"a", "b", "c"}, jobs, &out, func(string, *log.Logger) error {
		arrived.Done()
		select {
		case <-allArrived:
			return nil
		case <-time.After(5 * time.Second):
			return errors.New("the other files were not started")
		}
	})

	if !ok {
		t.Errorf("not run in parallel: %q", out.String())
	}
}

// No more than jobs files are in progress at once.
func TestForEachFileLimitsJobs(t *testing.T) {
	const jobs = 2

	var running, most atomic.Int32

	var out bytes.Buffer
	forEachFile([]string{"a", "b", "c", "d", "e", "f"}, jobs, &out, func(string, *log.Logger) error {
		now := running.Add(1)
		for {
			seen := most.Load()
			if now <= seen || most.CompareAndSwap(seen, now) {
				break
			}
		}

		time.Sleep(10 * time.Millisecond)
		running.Add(-1)
		return nil
	})

	if got := most.Load(); got > jobs {
		t.Errorf("%d files in progress at once, want at most %d", got, jobs)
	}
}

// A job count below one is treated as one, so the files are still done.
func TestForEachFileNeedsAtLeastOneJob(t *testing.T) {
	var done atomic.Int32

	var out bytes.Buffer
	forEachFile([]string{"a", "b"}, 0, &out, func(string, *log.Logger) error {
		done.Add(1)
		return nil
	})

	if got := done.Load(); got != 2 {
		t.Errorf("%d files were done, want 2", got)
	}
}
