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
	"strings"
	"time"
)

// DecompressFile takes a filename and decompresses it to a new file of the
// same name without the .gz suffix.  If keep is false, the original file is
// deleted if decompression is successful.
func DecompressFile(filename, suffix string, keep, force, verbose bool) (err error) {

	var (
		inputInfo  os.FileInfo
		outputInfo os.FileInfo
		inputFile  *os.File
		outputFile *renameio.PendingFile
		zr         *gzip.Reader

		outputFilename string
	)

	started := time.Now().UTC()

	if verbose {
		log.Printf(
			"Decompressing %s",
			filename,
		)
	}

	inputInfo, err = os.Stat(filename)
	if err != nil {
		return err
	}

	inputFile, err = os.Open(filename)
	if err != nil {
		return err
	}
	//goland:noinspection GoUnhandledErrorResult
	defer inputFile.Close()

	outputFilename = strings.TrimSuffix(filename, suffix)

	if !force {
		if _, err := os.Stat(outputFilename); err == nil {
			return errors.New("output file already exists")
		}
	}

	outputFile, err = renameio.TempFile(filepath.Dir(outputFilename), outputFilename)
	if err != nil {
		return err
	}
	//goland:noinspection GoUnhandledErrorResult
	defer outputFile.Cleanup()

	outputBuffer := bufio.NewWriter(outputFile)

	zr, err = gzip.NewReader(bufio.NewReader(inputFile))
	if err != nil {
		return err
	}

	_, err = io.Copy(outputBuffer, zr)
	if err != nil {
		return err
	}

	if err := zr.Close(); err != nil {
		return err
	}

	if err := inputFile.Close(); err != nil {
		return err
	}

	if err := outputBuffer.Flush(); err != nil {
		return errors.Wrap(err, "error flushing output buffer")
	}

	if err := outputFile.CloseAtomicallyReplace(); err != nil {
		return err
	}

	outputInfo, err = os.Stat(outputFilename)
	if err != nil {
		return err
	}

	if !keep {
		if err = os.Remove(filename); err != nil {
			return err
		}
	}

	if verbose {
		log.Printf(
			"Decompressed %s from %s to %s in %v",
			outputFilename,
			humanize.Bytes(uint64(inputInfo.Size())),
			humanize.Bytes(uint64(outputInfo.Size())),
			time.Since(started),
		)
	}

	return nil
}
