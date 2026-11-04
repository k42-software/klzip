// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"

	"golang.org/x/term"
)

// isTerminal reports whether stream is a terminal.  A stream that is not a
// file, such as a buffer, is not.  Tests replace it.
var isTerminal = func(stream any) bool {
	f, ok := stream.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
