// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"log"
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

// Options say what to do to some files.  They do not depend on how they were
// asked for.
type Options struct {
	Mode  Mode
	Files []string

	// Jobs is the most files processed at once.
	Jobs int

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

// Run does what the options say to each of the files, in parallel.  What the
// files log and their errors are written to w in the order of the files.  It
// reports whether every file succeeded.
func Run(opts Options, w io.Writer) bool {
	return forEachFile(opts.Files, opts.Jobs, w, func(file string, logger *log.Logger) error {
		if !opts.Verbose {
			logger = nil
		}
		return runFile(opts, file, logger)
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

func compress(opts Options, filename string, logger *log.Logger) error {
	// A file that already has the suffix is taken to be compressed already.
	if !opts.Force && strings.HasSuffix(filename, opts.Suffix) {
		return skipped{reason: "already has " + opts.Suffix + " suffix -- unchanged"}
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
