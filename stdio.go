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

	"github.com/klauspost/compress/gzip"
	"github.com/pkg/errors"
)

// outputBufferSize is the size of the buffer in front of each file that writes
// to stdout.  A file that is waiting for its turn can produce this much output
// ahead before it has to stop.
const outputBufferSize = 64 << 10

// input is what is read to write the result to stdout.
type input struct {
	*bufio.Reader

	// name and mtime are what the gzip header stores when compressing.  They are
	// empty for stdin.
	name  string
	mtime time.Time

	// file is the open file.  It is nil for stdin.
	file *os.File
}

// Close closes the file, if there is one.
func (in *input) Close() error {
	if in.file == nil {
		return nil
	}

	return in.file.Close()
}

// runToStdout does what the options say to one file, and writes the result to
// w.  The file is kept.  The turn of w is given up on every path out.
func runToStdout(opts Options, file string, w *turnWriter) error {
	defer w.Done()

	in, err := openInput(opts, file)
	if err != nil {
		return err
	}
	//goland:noinspection GoUnhandledErrorResult
	defer in.Close()

	output := bufio.NewWriterSize(w, outputBufferSize)

	if err := convertStream(opts, in, output); err != nil {
		return err
	}

	// This waits for the turn of w if it has not come yet.
	if err := output.Flush(); err != nil {
		return errors.Wrap(err, "error writing output")
	}

	return nil
}

// openInput opens file for reading, or stdin if file is stdinName.
func openInput(opts Options, file string) (*input, error) {
	if file == stdinName {
		if err := checkTerminals(opts); err != nil {
			return nil, err
		}

		return &input{Reader: bufio.NewReader(stdin)}, nil
	}

	if opts.Mode == ModeCompress {
		if err := skipIfCompressed(opts, file); err != nil {
			return nil, err
		}
	}

	f, err := os.Open(file)
	if err != nil {
		return nil, errors.Wrap(err, "error opening input file")
	}

	info, err := f.Stat()
	if err != nil {
		//goland:noinspection GoUnhandledErrorResult
		f.Close()
		return nil, errors.Wrap(err, "error stating input file")
	}

	in := &input{Reader: bufio.NewReader(f), file: f}
	if opts.Named {
		in.name, in.mtime = filepath.Base(file), info.ModTime()
	}

	return in, nil
}

// convertStream does what the options say to the data of in, and writes the
// result to w.
func convertStream(opts Options, in *input, w io.Writer) error {
	switch opts.Mode {
	case ModeCompress:
		if opts.Rsyncable {
			return xflateStream(in, w, opts.Level, in.name, in.mtime)
		}
		return compressStream(in, w, opts.Level, in.name, in.mtime)

	case ModeDecompress:
		zr, err := gzip.NewReader(in)
		if err != nil {
			return errors.Wrap(err, "error creating gzip reader")
		}
		return decompressStream(zr, w)

	default:
		return errors.Errorf("unknown mode %d", opts.Mode)
	}
}

// verifyStdin checks that stdin holds a gzip stream.
func verifyStdin(opts Options) error {
	if err := checkTerminals(opts); err != nil {
		return err
	}

	return verifyStream(bufio.NewReader(stdin))
}

// checkTerminals is for stdin.  Unless forced, it refuses to compress stdin to
// a terminal, and to decompress or test stdin when it is a terminal.
func checkTerminals(opts Options) error {
	if opts.Force {
		return nil
	}

	if opts.Mode == ModeCompress && isTerminal(stdout) {
		return errors.New("compressed data not written to a terminal. Use -f to force compression.")
	}

	if opts.Mode != ModeCompress && isTerminal(stdin) {
		return errors.New("compressed data not read from a terminal. Use -f to force decompression.")
	}

	return nil
}
