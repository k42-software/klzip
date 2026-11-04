// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command klzip compresses and decompresses files in the gzip format.
package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

//go:embed "LICENSE"
var license string

const (
	name        = "klzip"
	description = "Better faster gzip compression - Brian Wojtczak"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var cli CLI
	kctx := kong.Parse(&cli, kongOptions()...)

	if cli.Licence {
		printLicence()
		return
	}

	opts, err := cli.options()
	kctx.FatalIfErrorf(err)

	if !Run(opts) {
		os.Exit(1)
	}
}

// kongOptions returns the options for parsing the command line.
func kongOptions() []kong.Option {
	return []kong.Option{
		kong.Name(name),
		kong.Description(description),
		kong.UsageOnError(),
		kong.Vars{
			"version": name + " " + versionString(),
			"suffix":  defaultSuffix,
		},
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
	}
}

// versionString describes this build.
func versionString() string {
	return fmt.Sprintf("%s, commit %s, built at %s", version, commit, date)
}

// printLicence prints the version and the licence.
func printLicence() {
	fmt.Println(name, versionString())
	fmt.Println("License     : BSD-3-Clause license")
	fmt.Println()
	fmt.Println(license)
}
