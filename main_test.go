// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	stdgzip "compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// executeWithStdin does what main does with a command line, except exit.  It
// returns what main would have written to stdout.  If main would exit with a
// failure, the error holds what main would have written to stderr.  Keep it in
// step with main.
func executeWithStdin(t *testing.T, in string, args []string) (string, error) {
	t.Helper()

	out, errs := useStreams(t, strings.NewReader(in))

	cli, err := parseArgs(args)
	if err != nil {
		return "", err
	}

	opts, err := cli.options()
	if err != nil {
		return "", err
	}

	if !Run(opts) {
		return out.String(), errors.New(errs.String())
	}

	return out.String(), nil
}

// executeArgs is executeWithStdin with nothing to read on stdin, and without the
// output on stdout.
func executeArgs(t *testing.T, args []string) error {
	t.Helper()

	_, err := executeWithStdin(t, "", args)
	return err
}

// writeNamedGzip writes a gzip file at path whose header stores name.
func writeNamedGzip(t *testing.T, path, name, contents string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	zw := stdgzip.NewWriter(f)
	zw.Name = name
	if _, err := zw.Write([]byte(contents)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteStubs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"recursive", []string{"-r", "f"}, "--recursive is not implemented"},
		{"dev file", []string{"/dev/null"}, "dev devices"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executeArgs(t, tt.args)
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

	if err := executeArgs(t, []string{"-kv", path}); err != nil {
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
		if err := executeArgs(t, args); err != nil {
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

func TestExecuteSeveralFiles(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("some data\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	if err := executeArgs(t, append([]string{"-k", "-j", "2"}, paths...)); err != nil {
		t.Fatal(err)
	}

	for _, path := range paths {
		if _, err := os.Stat(path + ".gz"); err != nil {
			t.Errorf("expected %s.gz to exist: %v", filepath.Base(path), err)
		}
	}
}

// A file that fails is reported by name, and the other files are still done.
func TestExecuteFailedFileDoesNotStopOthers(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.txt")
	path := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(path, []byte("some data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := executeArgs(t, []string{"-j", "1", missing, path})
	if err == nil {
		t.Fatal("expected an error")
	}
	if want := "klzip: error: " + missing + ": "; !strings.HasPrefix(err.Error(), want) {
		t.Errorf("error = %q, want it to start with %q", err, want)
	}

	if _, err := os.Stat(path + ".gz"); err != nil {
		t.Errorf("the second file was not compressed: %v", err)
	}
}

// A file and its own output listed together: the output name already has the
// suffix, so it is skipped, and nothing is lost.
func TestExecuteFileAndItsOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("my new data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := []byte("an old archive")
	if err := os.WriteFile(path+".gz", old, 0o644); err != nil {
		t.Fatal(err)
	}

	err := executeArgs(t, []string{path, path + ".gz"})

	want := "klzip: error: " + path + ": output file already exists\nklzip: " + path + ".gz: already has .gz suffix -- unchanged\n"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}

	if got, err := os.ReadFile(path); err != nil || string(got) != "my new data\n" {
		t.Errorf("a.txt = %q, %v; want it unchanged", got, err)
	}
	if got, err := os.ReadFile(path + ".gz"); err != nil || !bytes.Equal(got, old) {
		t.Errorf("a.txt.gz = %q, %v; want it unchanged", got, err)
	}
	if _, err := os.Stat(path + ".gz.gz"); err == nil {
		t.Error("a.txt.gz.gz was created")
	}
}

func TestExecuteFileAndMissingOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("some data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := executeArgs(t, []string{path, path + ".gz"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path + ".gz"); err != nil {
		t.Errorf("a.txt was not compressed: %v", err)
	}
	if _, err := os.Stat(path + ".gz.gz"); err == nil {
		t.Error("a.txt.gz.gz was created")
	}
}

// Files that store the same name are decompressed one at a time with -N, so
// the first is converted, and the second is refused with its input kept.
func TestExecuteSharedStoredName(t *testing.T) {
	dir := t.TempDir()
	mon := filepath.Join(dir, "mon.gz")
	tue := filepath.Join(dir, "tue.gz")
	writeNamedGzip(t, mon, "backup.sql", "monday\n")
	writeNamedGzip(t, tue, "backup.sql", "tuesday\n")

	err := executeArgs(t, []string{"-d", "-N", mon, tue})
	if want := "klzip: error: " + tue + ": output file already exists"; err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("error = %v, want it to start with %q", err, want)
	}

	got, err := os.ReadFile(filepath.Join(dir, "backup.sql"))
	if err != nil || string(got) != "monday\n" {
		t.Errorf("backup.sql = %q, %v; want the first file's contents", got, err)
	}

	if _, err := os.Stat(mon); err == nil {
		t.Error("the first input was kept, want it removed")
	}
	if _, err := os.Stat(tue); err != nil {
		t.Errorf("the second input was not kept: %v", err)
	}
}
