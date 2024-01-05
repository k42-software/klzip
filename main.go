// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"github.com/devfacet/gocmd"
	"log"
	"os"
	"strings"
)

import _ "embed"

//go:embed "LICENSE"
var license string

var filename = "/dev/stdin"
var suffix = ".gz"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {

	if len(os.Args) > 0 {
		last := os.Args[len(os.Args)-1]
		if !strings.HasPrefix(last, "-") {
			filename = last
			os.Args = os.Args[:len(os.Args)-1]
		}
	}

	flags := struct {
		Stdout     bool   `short:"s" long:"stdout" description:"Write output to stdout instead of a file"`
		Decompress bool   `short:"d" long:"decompress" description:"Decompress the input file"`
		Force      bool   `short:"f" long:"force" description:"Force overwrite of output file"`
		Help       bool   `short:"h" long:"help" description:"Display usage"`
		Keep       bool   `short:"k" long:"keep" description:"Keep (don't delete) input files"`
		List       bool   `short:"l" long:"list" description:"[NOT IMPLEMENTED] List the contents of the compressed file"`
		Licence    bool   `short:"L" long:"licence" description:"Display licence"`
		NoName     bool   `short:"n" long:"no-name" description:"Do not save or restore the original filename and timestamp"`
		Name       bool   `short:"N" long:"name" description:"Save or restore the original filename and timestamp"`
		Quiet      bool   `short:"q" long:"quiet" description:"Suppress non-fatal error messages" default:"true"`
		Recursive  bool   `short:"r" long:"recursive" description:"[NOT IMPLEMENTED] Process directories recursively"`
		Rsyncable  bool   `short:"" long:"rsyncable" description:"Make rsync-friendly archive (xflate mode)"`
		Suffix     string `short:"S" long:"suffix" description:"Suffix for compressed files" default:".gz"`
		Test       bool   `short:"t" long:"test" description:"Test the integrity of the compressed file"`
		Verbose    bool   `short:"v" long:"verbose" description:"Verbose mode"`
		Version    bool   `short:"V" long:"version" description:"Display version"`

		LevelOne   bool `short:"1" long:"fast" description:"Compress level one (fastest)"`
		LevelTwo   bool `short:"2" long:"" description:"Compress level two"`
		LevelThree bool `short:"3" long:"" description:"Compress level three"`
		LevelFour  bool `short:"4" long:"" description:"Compress level four"`
		LevelFive  bool `short:"5" long:"" description:"Compress level five (reasonable)"`
		LevelSix   bool `short:"6" long:"" description:"Compress level six"`
		LevelSeven bool `short:"7" long:"" description:"Compress level seven"`
		LevelEight bool `short:"8" long:"" description:"Compress level eight"`
		LevelNine  bool `short:"9" long:"best" description:"Compress level nine (best)"`

		Filename string `short:"" long:"" description:"Filename to compress or decompress"`

		Settings bool `settings:"true" allow-unknown-arg:"false" global:"true"`
	}{}

	// Init the app
	cmd, err := gocmd.New(gocmd.Options{
		Name:        "klzip",
		Description: "Better faster gzip compression - Brian Wojtczak",
		Version:     fmt.Sprintf("%v, commit %v, built at %v", version, commit, date),
		Flags:       &flags,
		AnyError:    false,
		AutoHelp:    true,
		AutoVersion: false,
		ExitOnError: false,
	})
	if err != nil {
		log.Fatalf("ERROR: %s", err)
	}

	if cmd.FlagValue("Version").(bool) || cmd.FlagValue("Licence").(bool) {
		cmd.PrintVersion(true)
		fmt.Println("License     : BSD-3-Clause license")
		if cmd.FlagValue("Licence").(bool) {
			fmt.Println()
			fmt.Println(license)
		}
		return
	}

	if cmd.FlagValue("Help").(bool) || len(os.Args) == 0 {
		cmd.PrintUsage()
		return
	}

	if cmd.FlagValue("Stdout").(bool) {
		log.Fatal("NOTICE: Support for --stdout is not implemented")
	}
	if cmd.FlagValue("List").(bool) {
		log.Fatal("NOTICE: Support for --list is not implemented")
	}
	if cmd.FlagValue("Recursive").(bool) {
		log.Fatal("NOTICE: Support for --recursive is not implemented")
	}

	if strings.HasPrefix(filename, "/dev/") {
		err = errors.New("interacting dev devices is not implemented yet")
		log.Fatalf("ERROR: %s", err)
	}

	if len(cmd.FlagValue("Suffix").(string)) > 0 {
		suffix = cmd.FlagValue("Suffix").(string)
		if !strings.HasPrefix(suffix, ".") {
			suffix = "." + suffix
		}
	}

	switch {
	case cmd.FlagValue("Test").(bool):
		err = testHandler(cmd, os.Args)

	case cmd.FlagValue("Decompress").(bool):
		err = decompressHandler(cmd, os.Args)

	default:
		err = compressHandler(cmd, os.Args)

	}
	if err != nil {
		log.Fatalf("ERROR: %s", err)
	}
}

func getLevel(cmd *gocmd.Cmd) int {
	switch {
	case cmd.FlagValue("LevelOne").(bool):
		return 1
	case cmd.FlagValue("LevelTwo").(bool):
		return 2
	case cmd.FlagValue("LevelThree").(bool):
		return 3
	case cmd.FlagValue("LevelFour").(bool):
		return 4
	case cmd.FlagValue("LevelFive").(bool):
		return 5
	case cmd.FlagValue("LevelSix").(bool):
		return 6
	case cmd.FlagValue("LevelSeven").(bool):
		return 7
	case cmd.FlagValue("LevelEight").(bool):
		return 8
	case cmd.FlagValue("LevelNine").(bool):
		return 9
	default:
		return 5
	}
}

func compressHandler(cmd *gocmd.Cmd, args []string) error {

	level := getLevel(cmd)
	keep := cmd.FlagValue("Keep").(bool)
	force := cmd.FlagValue("Force").(bool)
	named := cmd.FlagValue("Name").(bool)
	notNamed := cmd.FlagValue("NoName").(bool)
	verbose := cmd.FlagValue("Verbose").(bool)

	if !named && !notNamed {
		named = true
	} else if named && notNamed {
		return errors.New("cannot specify both --name and --no-name")
	} else if notNamed {
		named = false
	}

	log.Println("DEBUG: filename:", filename)
	log.Println("DEBUG: suffix:", suffix)
	log.Println("DEBUG: level:", level)
	log.Println("DEBUG: keep:", keep)
	log.Println("DEBUG: force:", force)
	log.Println("DEBUG: named:", named)
	log.Println("DEBUG: verbose:", verbose)

	rsyncable := cmd.FlagValue("Rsyncable").(bool)
	log.Println("DEBUG: rsyncable:", rsyncable)
	if rsyncable {
		return XflateCompressFile(filename, suffix, level, keep, force, named, verbose)
	}

	return CompressFile(filename, suffix, level, keep, force, named, verbose)
}

func decompressHandler(cmd *gocmd.Cmd, args []string) error {

	keep := cmd.FlagValue("Keep").(bool)
	force := cmd.FlagValue("Force").(bool)
	verbose := cmd.FlagValue("Verbose").(bool)

	log.Println("DEBUG: filename:", filename)
	log.Println("DEBUG: suffix:", suffix)
	log.Println("DEBUG: keep:", keep)
	log.Println("DEBUG: force:", force)
	log.Println("DEBUG: verbose:", verbose)

	return DecompressFile(filename, suffix, keep, force, verbose)
}

func testHandler(cmd *gocmd.Cmd, args []string) error {
	verbose := cmd.FlagValue("Verbose").(bool)
	return TestFile(filename, verbose)
}
