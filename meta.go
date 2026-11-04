// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"time"

	"github.com/pkg/errors"
)

// copyMode sets the permission bits of the (not yet renamed) output file to
// match those of the input file.
func copyMode(output *os.File, input os.FileInfo) error {
	if err := output.Chmod(input.Mode().Perm()); err != nil {
		return errors.Wrap(err, "error setting output file mode")
	}
	return nil
}

// copyTimes sets the access and modification times of the file at path to mtime.
func copyTimes(outputFilename string, mtime time.Time) error {
	if err := os.Chtimes(outputFilename, mtime, mtime); err != nil {
		return errors.Wrap(err, "error setting output file times")
	}
	return nil
}
