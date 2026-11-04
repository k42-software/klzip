// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// executeArgs does what main does with a command line, except exit. Keep it in
// step with main.
func executeArgs(args []string) error {
	cli, err := parseArgs(args)
	if err != nil {
		return err
	}

	opts, err := cli.options()
	if err != nil {
		return err
	}

	return Run(opts)
}

func TestExecuteStubs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"stdout", []string{"-c", "f"}, "--stdout is not implemented"},
		{"list", []string{"-l", "f"}, "--list is not implemented"},
		{"recursive", []string{"-r", "f"}, "--recursive is not implemented"},
		{"no file", []string{"-k"}, "stdin is not implemented"},
		{"stdin marker", []string{"-"}, "stdin is not implemented"},
		{"dev file", []string{"/dev/null"}, "dev devices"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executeArgs(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want one mentioning %q", err, tt.want)
			}
		})
	}
}

// Combined short flags must be honoured: -kv keeps the input file.
func TestExecuteCombinedFlagsKeepInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(path, []byte("some data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := executeArgs([]string{"-kv", path}); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{path, path + ".gz"} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist: %v", filepath.Base(p), err)
		}
	}
}

// -S with a suffix must reach compress and decompress.
func TestExecuteCustomSuffixRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(path, []byte("some data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"-S", "foo", path}, {"-d", "-S", "foo", path + ".foo"}} {
		if err := executeArgs(args); err != nil {
			t.Fatalf("%q: %v", args, err)
		}
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "some data\n" {
		t.Errorf("content = %q", got)
	}
}
