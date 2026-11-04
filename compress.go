// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/klauspost/compress/gzip"
	"github.com/pkg/errors"
)

// CompressFile takes a filename and compresses it to a new file of the
// same name with the suffix added.  If keep is false, the original file is
// deleted if compression is successful.  A nil logger logs nothing.
func CompressFile(filename, suffix string, level int, keep, force, named bool, logger *log.Logger) error {

	started := time.Now().UTC()

	if logger != nil {
		logger.Printf(
			"Compressing %s",
			filename,
		)
	}

	res, err := convertFile(filename, keep, force, func(r io.Reader, info os.FileInfo) (*conversion, error) {
		return &conversion{
			outputFilename: filename + suffix,
			mtime:          info.ModTime(),
			run: func(w io.Writer) error {
				var name string
				var mtime time.Time
				if named {
					name, mtime = filepath.Base(filename), info.ModTime()
				}

				return compressStream(r, w, level, name, mtime)
			},
		}, nil
	})
	if err != nil {
		return err
	}

	if logger != nil {
		logger.Printf(
			"Compressed %s from %s to %s (level %d) in %v",
			res.outputFilename,
			humanize.Bytes(uint64(res.inputSize)),
			humanize.Bytes(uint64(res.outputSize)),
			level,
			time.Since(started),
		)
	}

	return nil
}

// compressStream compresses r to w as a gzip stream.  The gzip header stores
// name and mtime, and a zero mtime stores none.
func compressStream(r io.Reader, w io.Writer, level int, name string, mtime time.Time) error {
	zw, err := gzip.NewWriterLevel(w, level)
	if err != nil {
		return errors.Wrap(err, "error creating gzip writer")
	}

	zw.Name = name

	if mtime.IsZero() {
		// A zero time.Time has a large negative Unix time, which the writer
		// truncates into a bogus header mtime.  The Unix epoch encodes as 0,
		// meaning no mtime.
		mtime = time.Unix(0, 0)
	}
	zw.ModTime = mtime

	if _, err := io.Copy(zw, r); err != nil {
		return errors.Wrap(err, "error writing data")
	}

	if err := zw.Flush(); err != nil {
		return errors.Wrap(err, "error flushing data")
	}

	if err := zw.Close(); err != nil {
		return errors.Wrap(err, "error closing output writer")
	}

	return nil
}
