// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
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

// headerBaseName returns the final path element of a filename stored in a gzip
// header, or "" if the name is empty or is ".", ".." or "/".  Directory
// components of the stored name are dropped so that a hostile header cannot
// write outside the target directory.
func headerBaseName(name string) string {
	base := filepath.Base(filepath.FromSlash(name))
	if name == "" || base == "." || base == ".." || base == string(filepath.Separator) {
		return ""
	}
	return base
}
