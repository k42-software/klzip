// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

// parseArgs parses a command line, without running it.
func parseArgs(args []string) (*CLI, error) {
	var cli CLI
	parser, err := kong.New(&cli, kongOptions()...)
	if err != nil {
		return nil, err
	}
	if _, err := parser.Parse(args); err != nil {
		return nil, err
	}
	return &cli, nil
}

func wantFiles(t *testing.T, c *CLI, want ...string) {
	t.Helper()
	if len(want) == 0 && len(c.Files) == 0 {
		return
	}
	if !reflect.DeepEqual(c.Files, want) {
		t.Errorf("files = %q, want %q", c.Files, want)
	}
}

func wantSuffix(t *testing.T, c *CLI, want string) {
	t.Helper()
	if c.Suffix != want {
		t.Errorf("suffix = %q, want %q", c.Suffix, want)
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		check func(t *testing.T, cli *CLI)
	}{
		{"combined short flags", []string{"-kv", "f"}, func(t *testing.T, c *CLI) {
			if !c.Keep || !c.Verbose || c.Decompress {
				t.Errorf("keep=%v verbose=%v decompress=%v", c.Keep, c.Verbose, c.Decompress)
			}
			wantFiles(t, c, "f")
		}},
		{"combined decompress and keep", []string{"-dk", "f"}, func(t *testing.T, c *CLI) {
			if !c.Decompress || !c.Keep {
				t.Errorf("decompress=%v keep=%v", c.Decompress, c.Keep)
			}
		}},
		{"flag after file", []string{"f", "-k"}, func(t *testing.T, c *CLI) {
			if !c.Keep {
				t.Error("keep not set")
			}
			wantFiles(t, c, "f")
		}},
		{"suffix separate", []string{"-S", "gz", "f"}, func(t *testing.T, c *CLI) {
			wantSuffix(t, c, "gz")
			wantFiles(t, c, "f")
		}},
		{"suffix attached", []string{"-Sgz", "f"}, func(t *testing.T, c *CLI) { wantSuffix(t, c, "gz") }},
		{"suffix long", []string{"--suffix=gz", "f"}, func(t *testing.T, c *CLI) { wantSuffix(t, c, "gz") }},
		{"double dash ends flags", []string{"-k", "--", "-weird"}, func(t *testing.T, c *CLI) {
			if !c.Keep {
				t.Error("keep not set")
			}
			wantFiles(t, c, "-weird")
		}},
		{"stdin marker is a file", []string{"-"}, func(t *testing.T, c *CLI) { wantFiles(t, c, "-") }},
		{"no files", []string{"-k"}, func(t *testing.T, c *CLI) { wantFiles(t, c) }},
		{"long aliases", []string{"--to-stdout", "--license", "--rsyncable", "f"}, func(t *testing.T, c *CLI) {
			if !c.Stdout || !c.Licence || !c.Rsyncable {
				t.Errorf("stdout=%v licence=%v rsyncable=%v", c.Stdout, c.Licence, c.Rsyncable)
			}
		}},
		{"name", []string{"-N", "f"}, func(t *testing.T, c *CLI) {
			if !c.Name || c.NoName {
				t.Errorf("name=%v noname=%v", c.Name, c.NoName)
			}
		}},
		{"no-name", []string{"--no-name", "f"}, func(t *testing.T, c *CLI) {
			if c.Name || !c.NoName {
				t.Errorf("name=%v noname=%v", c.Name, c.NoName)
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli, err := parseArgs(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, cli)
		})
	}
}

func TestParseArgsErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unknown long flag", []string{"--bogus", "f"}, "unknown flag --bogus"},
		{"unknown short flag", []string{"-z", "f"}, "unknown flag -z"},
		{"unknown short flag in group", []string{"-kz", "f"}, "unknown flag -z"},
		{"missing suffix value", []string{"-S"}, "--suffix"},
		{"several files", []string{"a", "b"}, "only one file is supported"},
		{"name and no-name", []string{"-n", "-N", "f"}, "--no-name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseArgs(tt.args)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}

func TestLevel(t *testing.T) {
	tests := []struct {
		args []string
		want int
	}{
		{[]string{"f"}, 5},
		{[]string{"-1", "f"}, 1},
		{[]string{"--fast", "f"}, 1},
		{[]string{"-6", "f"}, 6},
		{[]string{"-9", "f"}, 9},
		{[]string{"--best", "f"}, 9},
		// The lowest level wins when several are given.
		{[]string{"-1", "-9", "f"}, 1},
		{[]string{"-9", "-3", "f"}, 3},
		{[]string{"-19", "f"}, 1},
	}

	for _, tt := range tests {
		cli, err := parseArgs(tt.args)
		if err != nil {
			t.Fatalf("%q: %v", tt.args, err)
		}
		if got := cli.level(); got != tt.want {
			t.Errorf("%q: level = %d, want %d", tt.args, got, tt.want)
		}
	}
}

func TestSuffix(t *testing.T) {
	tests := map[string]string{
		"":     ".gz",
		"gz":   ".gz",
		".gz":  ".gz",
		"tgz":  ".tgz",
		".tgz": ".tgz",
	}

	for in, want := range tests {
		c := &CLI{Suffix: in}
		if got := c.suffix(); got != want {
			t.Errorf("suffix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want Options
	}{
		{"compress defaults", []string{"f"}, Options{
			Mode: ModeCompress, Filename: "f", Suffix: ".gz", Level: 5, Named: true,
		}},
		{"compress everything", []string{"-k", "-f", "-v", "-9", "-S", "foo", "--rsyncable", "f"}, Options{
			Mode: ModeCompress, Filename: "f", Suffix: ".foo", Level: 9, Named: true,
			Keep: true, Force: true, Verbose: true, Rsyncable: true,
		}},
		// Compressed files are named unless -n is given.
		{"compress no-name", []string{"-n", "f"}, Options{
			Mode: ModeCompress, Filename: "f", Suffix: ".gz", Level: 5,
		}},
		// Names are only restored when decompressing if -N is given.
		{"decompress", []string{"-d", "f.gz"}, Options{
			Mode: ModeDecompress, Filename: "f.gz", Suffix: ".gz", Level: 5,
		}},
		{"decompress name", []string{"-d", "-N", "f.gz"}, Options{
			Mode: ModeDecompress, Filename: "f.gz", Suffix: ".gz", Level: 5, Named: true,
		}},
		{"test", []string{"-t", "f.gz"}, Options{
			Mode: ModeTest, Filename: "f.gz", Suffix: ".gz", Level: 5, Named: true,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli, err := parseArgs(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			got, err := cli.options()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("options = %+v, want %+v", got, tt.want)
			}
		})
	}
}
