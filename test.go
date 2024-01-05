// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"github.com/dustin/go-humanize"
	"github.com/klauspost/compress/gzip"
	"github.com/pkg/errors"
	"io"
	"log"
	"os"
	"time"
)

// TestFile takes a filename and tests it for gzip integrity.
func TestFile(filename string, verbose bool) (err error) {

	var (
		inputInfo os.FileInfo
		inputFile *os.File
		zr        *gzip.Reader
	)

	started := time.Now().UTC()

	if verbose {
		log.Printf(
			"Testing %s",
			filename,
		)
	}

	inputInfo, err = os.Stat(filename)
	if err != nil {
		return errors.Wrap(err, "error stating input file")
	}

	inputFile, err = os.Open(filename)
	if err != nil {
		return errors.Wrap(err, "error opening input file")
	}
	//goland:noinspection GoUnhandledErrorResult
	defer inputFile.Close()

	zr, err = gzip.NewReader(bufio.NewReader(inputFile))
	if err != nil {
		return errors.Wrap(err, "error creating gzip reader")
	}

	_, err = io.Copy(io.Discard, zr)
	if err != nil {
		return errors.Wrap(err, "error decompressing file")
	}

	if err := zr.Close(); err != nil {
		return errors.Wrap(err, "error closing gzip reader")
	}

	if err := inputFile.Close(); err != nil {
		return errors.Wrap(err, "error closing input file")
	}

	if verbose {
		log.Printf(
			"Tested %s (%s) in %v",
			filename,
			humanize.Bytes(uint64(inputInfo.Size())),
			time.Since(started),
		)
	}

	return nil
}
