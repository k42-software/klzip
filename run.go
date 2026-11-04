// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"

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

// Options say what to do to one file.  They do not depend on how they were
// asked for.
type Options struct {
	Mode     Mode
	Filename string

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

// Run does what the options say.
func Run(opts Options) error {
	switch opts.Mode {
	case ModeCompress:
		return compress(opts)
	case ModeDecompress:
		return decompress(opts)
	case ModeTest:
		return VerifyFile(opts.Filename, opts.Verbose)
	default:
		return errors.Errorf("unknown mode %d", opts.Mode)
	}
}

func compress(opts Options) error {
	if opts.Verbose {
		log.Println("DEBUG: filename:", opts.Filename)
		log.Println("DEBUG: suffix:", opts.Suffix)
		log.Println("DEBUG: level:", opts.Level)
		log.Println("DEBUG: keep:", opts.Keep)
		log.Println("DEBUG: force:", opts.Force)
		log.Println("DEBUG: named:", opts.Named)
		log.Println("DEBUG: rsyncable:", opts.Rsyncable)
	}

	if opts.Rsyncable {
		return XflateCompressFile(opts.Filename, opts.Suffix, opts.Level, opts.Keep, opts.Force, opts.Named, opts.Verbose)
	}

	return CompressFile(opts.Filename, opts.Suffix, opts.Level, opts.Keep, opts.Force, opts.Named, opts.Verbose)
}

func decompress(opts Options) error {
	if opts.Verbose {
		log.Println("DEBUG: filename:", opts.Filename)
		log.Println("DEBUG: suffix:", opts.Suffix)
		log.Println("DEBUG: keep:", opts.Keep)
		log.Println("DEBUG: force:", opts.Force)
		log.Println("DEBUG: named:", opts.Named)
	}

	return DecompressFile(opts.Filename, opts.Suffix, opts.Keep, opts.Force, opts.Named, opts.Verbose)
}
