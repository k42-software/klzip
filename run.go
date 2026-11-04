// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
)

// Mode is what klzip does to a file.
type Mode int

const (
	// ModeCompress compresses the file.
	ModeCompress Mode = iota

	// ModeDecompress decompresses the file.
	ModeDecompress

	// ModeTest checks the integrity of a compressed file.
	ModeTest
)

// stdinName is the name in Options.Files that stands for stdin.
const stdinName = "-"

// The standard streams.  Tests replace them.
var (
	stdin  io.Reader = os.Stdin
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// Options say what to do to some files.  They do not depend on how they were
// asked for.
type Options struct {
	Mode  Mode
	Files []string

	// Jobs is the most files processed at once.
	Jobs int

	// Stdout writes the result for each file to stdout, in the order of the
	// files, and keeps the files.  It is not used to test.
	Stdout bool

	// Suffix is the suffix of compressed files, with its leading dot.
	Suffix string

	// Level is the compression level.  It is only used to compress.
	Level int
	Keep  bool
	Force bool

	// Named saves the original name and mtime when compressing, and restores
	// them when decompressing.
	Named bool

	// Rsyncable compresses to an rsync-friendly archive.  It is only used to
	// compress.
	Rsyncable bool
	Verbose   bool
}

// writesToStdout reports whether the result for file is written to stdout.
func (opts Options) writesToStdout(file string) bool {
	return opts.Mode != ModeTest && (opts.Stdout || file == stdinName)
}

// Run does what the options say to each of the files, in parallel.  A file
// named stdinName is read from stdin.  What the files log and their errors are
// written to stderr in the order of the files.  It reports whether every file
// succeeded.
func Run(opts Options) bool {
	// The files that write to stdout take turns, in the order of the files.
	positions := make(map[string]int)
	for _, file := range opts.Files {
		if opts.writesToStdout(file) {
			positions[file] = len(positions)
		}
	}
	turns := newSequencer(stdout, len(positions))

	return forEachFile(opts.Files, opts.Jobs, stderr, func(file string, logger *log.Logger) error {
		if !opts.Verbose {
			logger = nil
		}

		switch position, ok := positions[file]; {
		case ok:
			return runToStdout(opts, file, turns.writer(position))
		case file == stdinName:
			return verifyStdin(opts)
		default:
			return runFile(opts, file, logger)
		}
	})
}

// runFile does what the options say to one file.  A nil logger logs nothing.
func runFile(opts Options, filename string, logger *log.Logger) error {
	switch opts.Mode {
	case ModeCompress:
		return compress(opts, filename, logger)
	case ModeDecompress:
		return decompress(opts, filename, logger)
	case ModeTest:
		return VerifyFile(filename, logger)
	default:
		return errors.Errorf("unknown mode %d", opts.Mode)
	}
}

// displayName returns the name to show for file in messages.
func displayName(file string) string {
	if file == stdinName {
		return "stdin"
	}

	return file
}

// skipIfCompressed returns skipped for a file that already has the suffix, and
// so is taken to be compressed already, unless the options force it.
func skipIfCompressed(opts Options, filename string) error {
	if !opts.Force && strings.HasSuffix(filename, opts.Suffix) {
		return skipped{reason: "already has " + opts.Suffix + " suffix -- unchanged"}
	}

	return nil
}

// compress compresses the file to a new file with the suffix added.  A file that
// already has the suffix is skipped, unless the options force it.  A nil logger
// logs nothing.
func compress(opts Options, filename string, logger *log.Logger) error {
	if err := skipIfCompressed(opts, filename); err != nil {
		return err
	}

	if logger != nil {
		logger.Println("DEBUG: filename:", filename)
		logger.Println("DEBUG: suffix:", opts.Suffix)
		logger.Println("DEBUG: level:", opts.Level)
		logger.Println("DEBUG: keep:", opts.Keep)
		logger.Println("DEBUG: force:", opts.Force)
		logger.Println("DEBUG: named:", opts.Named)
		logger.Println("DEBUG: rsyncable:", opts.Rsyncable)
	}

	if opts.Rsyncable {
		return XflateCompressFile(filename, opts.Suffix, opts.Level, opts.Keep, opts.Force, opts.Named, logger)
	}

	return CompressFile(filename, opts.Suffix, opts.Level, opts.Keep, opts.Force, opts.Named, logger)
}

// decompress decompresses the file to a new file, named as the options say.  A
// nil logger logs nothing.
func decompress(opts Options, filename string, logger *log.Logger) error {
	if logger != nil {
		logger.Println("DEBUG: filename:", filename)
		logger.Println("DEBUG: suffix:", opts.Suffix)
		logger.Println("DEBUG: keep:", opts.Keep)
		logger.Println("DEBUG: force:", opts.Force)
		logger.Println("DEBUG: named:", opts.Named)
	}

	return DecompressFile(filename, opts.Suffix, opts.Keep, opts.Force, opts.Named, logger)
}
