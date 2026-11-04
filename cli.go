// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
)

const (
	// defaultSuffix is the suffix added to compressed files.
	defaultSuffix = ".gz"

	// defaultLevel is the compression level used when no level flag is given.
	defaultLevel = 5
)

// CLI is the command line of klzip.
type CLI struct {
	Stdout     bool   `short:"c" help:"Write output to stdout, and keep the input files" aliases:"to-stdout"`
	Decompress bool   `short:"d" help:"Decompress the input file"`
	Force      bool   `short:"f" help:"Force overwrite of output file"`
	Jobs       int    `short:"j" help:"Number of files to process at once (default: one per CPU, or 1 with -d -N)"`
	Keep       bool   `short:"k" help:"Keep (don't delete) input files"`
	List       bool   `short:"l" help:"[NOT IMPLEMENTED] List the contents of the compressed file"`
	Licence    bool   `short:"L" help:"Display licence" aliases:"license"`
	NoName     bool   `short:"n" name:"no-name" xor:"name" help:"Do not save or restore the original filename and timestamp"`
	Name       bool   `short:"N" xor:"name" help:"Save or restore the original filename and timestamp"`
	Quiet      bool   `short:"q" help:"Accepted for compatibility with gzip; has no effect"`
	Recursive  bool   `short:"r" help:"[NOT IMPLEMENTED] Process directories recursively"`
	Rsyncable  bool   `help:"Make rsync-friendly archive (xflate mode)"`
	Suffix     string `short:"S" default:"${suffix}" help:"Suffix for compressed files"`
	Test       bool   `short:"t" help:"Test the integrity of the compressed file"`
	Verbose    bool   `short:"v" help:"Verbose mode"`

	Version kong.VersionFlag `short:"V" help:"Display version"`

	Level1 bool `short:"1" name:"fast" help:"Compress level one (fastest)"`
	Level2 bool `short:"2" name:"level-2" hidden:"" help:"Compress level two"`
	Level3 bool `short:"3" name:"level-3" hidden:"" help:"Compress level three"`
	Level4 bool `short:"4" name:"level-4" hidden:"" help:"Compress level four"`
	Level5 bool `short:"5" name:"level-5" hidden:"" help:"Compress level five (default)"`
	Level6 bool `short:"6" name:"level-6" hidden:"" help:"Compress level six"`
	Level7 bool `short:"7" name:"level-7" hidden:"" help:"Compress level seven"`
	Level8 bool `short:"8" name:"level-8" hidden:"" help:"Compress level eight"`
	Level9 bool `short:"9" name:"best" help:"Compress level nine (best)"`

	Files []string `arg:"" optional:"" name:"file" help:"Files to compress or decompress (default: stdin)"`
}

// Validate is called by kong after the command line has been parsed.
func (c *CLI) Validate() error {
	if c.Jobs < 0 {
		return errors.New("--jobs must not be negative")
	}
	return nil
}

// level returns the compression level.  When several level flags are given,
// the lowest wins.
func (c *CLI) level() int {
	levels := []bool{
		c.Level1, c.Level2, c.Level3, c.Level4, c.Level5,
		c.Level6, c.Level7, c.Level8, c.Level9,
	}
	for i, set := range levels {
		if set {
			return i + 1
		}
	}
	return defaultLevel
}

// suffix returns the suffix for compressed files, with its leading dot.
func (c *CLI) suffix() string {
	if c.Suffix == "" {
		return defaultSuffix
	}
	if !strings.HasPrefix(c.Suffix, ".") {
		return "." + c.Suffix
	}
	return c.Suffix
}

// jobs returns how many files to process at once.  Without -j, or with -j 0, it
// is one per CPU, as limited by GOMAXPROCS.  When decompressing with -N it is
// one, since the names stored in two files can be the same, and then only the
// first file may write that output file.
func (c *CLI) jobs() int {
	switch {
	case c.Jobs > 0:
		return c.Jobs
	case c.Decompress && c.Name && !c.Test:
		return 1
	default:
		return runtime.GOMAXPROCS(0)
	}
}

// options checks that the command line asks for something klzip can do, and
// returns what to do.
func (c *CLI) options() (Options, error) {
	switch {
	case c.List:
		return Options{}, errors.New("--list is not implemented")
	case c.Recursive:
		return Options{}, errors.New("--recursive is not implemented")
	}

	for _, file := range c.Files {
		if strings.HasPrefix(file, "/dev/") {
			return Options{}, errors.New("interacting with dev devices is not implemented yet")
		}
	}

	// Without files, stdin is read.
	files := c.Files
	if len(files) == 0 {
		files = []string{stdinName}
	}

	opts := Options{
		Mode:      ModeCompress,
		Files:     uniqueFiles(files),
		Jobs:      c.jobs(),
		Stdout:    c.Stdout,
		Suffix:    c.suffix(),
		Level:     c.level(),
		Keep:      c.Keep,
		Force:     c.Force,
		Named:     !c.NoName, // Compressed files are named by default.
		Rsyncable: c.Rsyncable,
		Verbose:   c.Verbose,
	}

	switch {
	case c.Test:
		opts.Mode = ModeTest
	case c.Decompress:
		opts.Mode = ModeDecompress
		opts.Named = c.Name // Names are only restored when asked for.
	}

	return opts, nil
}

// uniqueFiles returns the files without repeats, in their original order.
// Two names are the same file when they have the same absolute path once
// tidied, so "a" and "./a" match.  The first spelling of each is kept.
func uniqueFiles(files []string) []string {
	seen := make(map[string]bool, len(files))
	unique := make([]string, 0, len(files))

	for _, file := range files {
		key, err := filepath.Abs(file)
		if err != nil {
			key = filepath.Clean(file)
		}

		if !seen[key] {
			seen[key] = true
			unique = append(unique, file)
		}
	}

	return unique
}
