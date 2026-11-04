// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/klauspost/compress/gzip"
	"github.com/pkg/errors"
)

// DecompressFile takes a filename and decompresses it to a new file of the
// same name without the .gz suffix.  If named is true, the original name and
// modification time stored in the file are restored when present.  If keep is
// false, the original file is deleted if decompression is successful.
func DecompressFile(filename, suffix string, keep, force, named, verbose bool) error {

	started := time.Now().UTC()

	if verbose {
		log.Printf(
			"Decompressing %s",
			filename,
		)
	}

	res, err := convertFile(filename, keep, force, func(r io.Reader, info os.FileInfo) (*conversion, error) {
		zr, err := gzip.NewReader(r)
		if err != nil {
			return nil, errors.Wrap(err, "error creating gzip reader")
		}

		// By default the output takes the name (suffix removed) and mtime of the
		// compressed file.  With named, the name and mtime stored in the gzip
		// header are used when present.
		outputFilename := strings.TrimSuffix(filename, suffix)
		mtime := info.ModTime()
		if named {
			if base := headerBaseName(zr.Name); base != "" {
				outputFilename = filepath.Join(filepath.Dir(filename), base)
			}
			// The reader reports a header time of 0 (not recorded) as the Unix
			// epoch, so test for that.
			if zr.ModTime.Unix() > 0 {
				mtime = zr.ModTime
			}
		}

		return &conversion{
			outputFilename: outputFilename,
			mtime:          mtime,
			run: func(w io.Writer) error {
				if _, err := io.Copy(w, zr); err != nil {
					return errors.Wrap(err, "error decompressing data")
				}

				if err := zr.Close(); err != nil {
					return errors.Wrap(err, "error closing gzip reader")
				}

				return nil
			},
		}, nil
	})
	if err != nil {
		return err
	}

	if verbose {
		log.Printf(
			"Decompressed %s from %s to %s in %v",
			res.outputFilename,
			humanize.Bytes(uint64(res.inputSize)),
			humanize.Bytes(uint64(res.outputSize)),
			time.Since(started),
		)
	}

	return nil
}
