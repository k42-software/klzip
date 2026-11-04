// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestRunUnknownMode(t *testing.T) {
	if err := Run(Options{Mode: Mode(99)}); err == nil {
		t.Error("expected an error")
	}
}
