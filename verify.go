// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/klauspost/compress/gzip"
	"github.com/pkg/errors"
)

// VerifyFile takes a filename and checks it for gzip integrity.  A nil logger
// logs nothing.
func VerifyFile(filename string, logger *log.Logger) (err error) {

	var (
		inputInfo os.FileInfo
		inputFile *os.File
	)

	started := time.Now().UTC()

	if logger != nil {
		logger.Printf(
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

	if err = verifyStream(bufio.NewReader(inputFile)); err != nil {
		return err
	}

	if err := inputFile.Close(); err != nil {
		return errors.Wrap(err, "error closing input file")
	}

	if logger != nil {
		logger.Printf(
			"Tested %s (%s) in %v",
			filename,
			humanize.Bytes(uint64(inputInfo.Size())),
			time.Since(started),
		)
	}

	return nil
}

// verifyStream reads the gzip stream r to its end, and checks its trailer.
func verifyStream(r io.Reader) error {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return errors.Wrap(err, "error creating gzip reader")
	}

	if _, err := io.Copy(io.Discard, zr); err != nil {
		return errors.Wrap(err, "error decompressing file")
	}

	if err := zr.Close(); err != nil {
		return errors.Wrap(err, "error closing gzip reader")
	}

	return nil
}
