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

// DecompressFile takes a filename and decompresses it to a new file.  The new
// file is named by taking the suffix of a compressed file off the name, or by
// replacing .tgz or .taz with .tar.  The suffix is one more suffix of
// compressed files, besides the usual ones.  If named is true, the original
// name and modification time stored in the file are restored when present.  If
// keep is false, the original file is deleted if decompression is successful.
// A nil logger logs nothing.
func DecompressFile(filename, suffix string, keep, force, named bool, logger *log.Logger) error {

	started := time.Now().UTC()

	if logger != nil {
		logger.Printf(
			"Decompressing %s",
			filename,
		)
	}

	res, err := convertFile(filename, keep, force, func(r io.Reader, info os.FileInfo) (*conversion, error) {
		zr, err := gzip.NewReader(r)
		if err != nil {
			return nil, errors.Wrap(err, "error creating gzip reader")
		}

		// By default the output takes the name that decompressedName gives the
		// compressed file, and its mtime.  With named, the name and mtime stored
		// in the gzip header are used when present, and then the suffix does not
		// matter.
		outputFilename, known := decompressedName(filename, suffix)
		mtime := info.ModTime()
		if named {
			if base := headerBaseName(zr.Name); base != "" {
				outputFilename = filepath.Join(filepath.Dir(filename), base)
				known = true
			}
			// The reader reports a header time of 0 (not recorded) as the Unix
			// epoch, so test for that.
			if zr.ModTime.Unix() > 0 {
				mtime = zr.ModTime
			}
		}

		if !known {
			return nil, errors.New("unknown suffix")
		}

		return &conversion{
			outputFilename: outputFilename,
			mtime:          mtime,
			run: func(w io.Writer) error {
				return decompressStream(zr, w)
			},
		}, nil
	})
	if err != nil {
		return err
	}

	if logger != nil {
		logger.Printf(
			"Decompressed %s from %s to %s in %v",
			res.outputFilename,
			humanize.Bytes(uint64(res.inputSize)),
			humanize.Bytes(uint64(res.outputSize)),
			time.Since(started),
		)
	}

	return nil
}

// decompressStream copies the decompressed data of the gzip stream zr to w.
// Reading the stream to its end checks its trailer, so a corrupt stream is an
// error.
func decompressStream(zr *gzip.Reader, w io.Writer) error {
	if _, err := io.Copy(w, zr); err != nil {
		return errors.Wrap(err, "error decompressing data")
	}

	if err := zr.Close(); err != nil {
		return errors.Wrap(err, "error closing gzip reader")
	}

	return nil
}
