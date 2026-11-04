// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/renameio"
	"github.com/pkg/errors"
)

// conversion describes how one input file becomes one output file.
type conversion struct {
	// outputFilename is the path of the file to create.
	outputFilename string

	// mtime is the modification time given to the output file.
	mtime time.Time

	// run writes the converted data to w.  It must flush any encoder it
	// created, but not w itself.
	run func(w io.Writer) error
}

// conversionResult reports what convertFile produced.
type conversionResult struct {
	outputFilename string
	inputSize      int64
	outputSize     int64
}

// convertFile converts the file filename into the file that prepare chooses,
// replacing it atomically and giving it the input's permission bits.  prepare
// is called with a reader for the input and its file info.  If keep is false,
// the input is removed once the output is in place.  If force is false, an
// existing output file is an error.  The output is never allowed to be the
// input file itself.
func convertFile(
	filename string,
	keep, force bool,
	prepare func(r io.Reader, info os.FileInfo) (*conversion, error),
) (res conversionResult, err error) {

	inputInfo, err := os.Stat(filename)
	if err != nil {
		return res, errors.Wrap(err, "error stating input file")
	}

	inputFile, err := os.Open(filename)
	if err != nil {
		return res, errors.Wrap(err, "error opening input file")
	}
	//goland:noinspection GoUnhandledErrorResult
	defer inputFile.Close()

	c, err := prepare(bufio.NewReader(inputFile), inputInfo)
	if err != nil {
		return res, err
	}

	// Refuse if the output is the input file itself.  A hard link, or a stored
	// name that differs only in case on a case-insensitive filesystem, can
	// give the output the input's identity under another name.
	if existing, err := os.Stat(c.outputFilename); err == nil {
		if os.SameFile(inputInfo, existing) {
			return res, errors.Errorf("%s: output would overwrite the input file", filename)
		}
		if !force {
			return res, errors.New("output file already exists")
		}
	} else if !os.IsNotExist(err) {
		return res, errors.Wrap(err, "error stating output file")
	}

	outputFile, err := renameio.TempFile(filepath.Dir(c.outputFilename), c.outputFilename)
	if err != nil {
		return res, errors.Wrap(err, "error creating temporary output file")
	}
	//goland:noinspection GoUnhandledErrorResult
	defer outputFile.Cleanup()

	if err := copyMode(outputFile.File, inputInfo); err != nil {
		return res, err
	}

	outputBuffer := bufio.NewWriter(outputFile)

	if err := c.run(outputBuffer); err != nil {
		return res, err
	}

	if err := inputFile.Close(); err != nil {
		return res, errors.Wrap(err, "error closing input file")
	}

	if err := outputBuffer.Flush(); err != nil {
		return res, errors.Wrap(err, "error flushing output buffer")
	}

	if err := copyTimes(outputFile.Name(), c.mtime); err != nil {
		return res, err
	}

	if err := outputFile.CloseAtomicallyReplace(); err != nil {
		return res, errors.Wrap(err, "error atomically closing output file")
	}

	outputInfo, err := os.Stat(c.outputFilename)
	if err != nil {
		return res, errors.Wrap(err, "error stating output file")
	}

	if !keep {
		if err := os.Remove(filename); err != nil {
			return res, errors.Wrap(err, "error removing input file")
		}
	}

	return conversionResult{
		outputFilename: c.outputFilename,
		inputSize:      inputInfo.Size(),
		outputSize:     outputInfo.Size(),
	}, nil
}
