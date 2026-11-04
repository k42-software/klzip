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
// same name without the .gz suffix.  If named is true, the original name and
// modification time stored in the file are restored when present.  If keep is
// false, the original file is deleted if decompression is successful.
func DecompressFile(filename, suffix string, keep, force, named, verbose bool) (err error) {

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

	zr, err = gzip.NewReader(bufio.NewReader(inputFile))
	if err != nil {
		return err
	}

	// By default the output takes the name (suffix removed) and mtime of the
	// compressed file.  With named, the name and mtime stored in the gzip
	// header are used when present.
	outputFilename = strings.TrimSuffix(filename, suffix)
	mtime := inputInfo.ModTime()
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

	// Refuse if the output is the input file itself.  With -N the output name
	// comes from the header, so it can reach the input through a hard link or,
	// on a case-insensitive filesystem, a name that differs only in case.
	if existing, err := os.Stat(outputFilename); err == nil {
		if os.SameFile(inputInfo, existing) {
			return errors.Errorf("%s: output would overwrite the input file", filename)
		}
		if !force {
			return errors.New("output file already exists")
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	outputFile, err = renameio.TempFile(filepath.Dir(outputFilename), outputFilename)
	if err != nil {
		return err
	}
	//goland:noinspection GoUnhandledErrorResult
	defer outputFile.Cleanup()

	if err := copyMode(outputFile.File, inputInfo); err != nil {
		return err
	}

	outputBuffer := bufio.NewWriter(outputFile)

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

	if err := copyTimes(outputFile.Name(), mtime); err != nil {
		return err
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
