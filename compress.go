// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"github.com/dustin/go-humanize"
	"github.com/google/renameio"
	"github.com/klauspost/compress/gzip"
	"github.com/pkg/errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// CompressFile takes a filename and compresses it to a new file of the
// same name with a .gz suffix added.  If keep is false, the original file is
// deleted if compression is successful.
func CompressFile(filename, suffix string, level int, keep, force, named, verbose bool) (err error) {

	var (
		inputInfo  os.FileInfo
		outputInfo os.FileInfo
		inputFile  *os.File
		outputFile *renameio.PendingFile
		zw         *gzip.Writer

		outputFilename string
	)

	started := time.Now().UTC()

	if verbose {
		log.Printf(
			"Compressing %s",
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

	outputFilename = filename + suffix

	if !force {
		if _, err := os.Stat(outputFilename); err == nil {
			return errors.New("output file already exists")
		}
	}

	outputFile, err = renameio.TempFile(filepath.Dir(outputFilename), outputFilename)
	if err != nil {
		return errors.Wrap(err, "error creating temporary output file")
	}
	//goland:noinspection GoUnhandledErrorResult
	defer outputFile.Cleanup()

	outputBuffer := bufio.NewWriter(outputFile)

	zw, err = gzip.NewWriterLevel(outputBuffer, level)
	if err != nil {
		return errors.Wrap(err, "error creating gzip writer")
	}

	if named {
		zw.Name = filepath.Base(filename)
		zw.ModTime = inputInfo.ModTime()
	}

	_, err = io.Copy(zw, bufio.NewReader(inputFile))
	if err != nil {
		return errors.Wrap(err, "error writing data")
	}

	if err := inputFile.Close(); err != nil {
		return errors.Wrap(err, "error closing input file")
	}

	if err := zw.Flush(); err != nil {
		return errors.Wrap(err, "error flushing data")
	}

	if err := zw.Close(); err != nil {
		return errors.Wrap(err, "error closing output writer")
	}

	if err := outputBuffer.Flush(); err != nil {
		return errors.Wrap(err, "error flushing output buffer")
	}

	if err := outputFile.CloseAtomicallyReplace(); err != nil {
		return errors.Wrap(err, "error atomically closing output file")
	}

	outputInfo, err = os.Stat(outputFilename)
	if err != nil {
		return errors.Wrap(err, "error stating output file")
	}

	if !keep {
		if err = os.Remove(filename); err != nil {
			return errors.Wrap(err, "error removing input file")
		}
	}

	if verbose {
		duration := time.Since(started)
		log.Printf(
			"Compressed %s from %s to %s (level %d) in %v",
			outputFilename,
			humanize.Bytes(uint64(inputInfo.Size())),
			humanize.Bytes(uint64(outputInfo.Size())),
			level,
			duration,
		)
	}

	return nil
}
