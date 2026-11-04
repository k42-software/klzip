// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	stdgzip "compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gunzip decompresses gzip data, which can hold several streams one after the
// other, and returns the result and the header of the first stream.
func gunzip(t *testing.T, data string) (string, stdgzip.Header) {
	t.Helper()

	zr, err := stdgzip.NewReader(strings.NewReader(data))
	if err != nil {
		t.Fatalf("output is not gzip: %v", err)
	}

	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("output is not valid gzip: %v", err)
	}

	return string(out), zr.Header
}

// gzipBytes compresses contents to a gzip stream.
func gzipBytes(t *testing.T, contents string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := stdgzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(contents)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

// mustGunzip is gunzip without the header.
func mustGunzip(t *testing.T, data string) string {
	t.Helper()

	out, _ := gunzip(t, data)
	return out
}

// useTerminals makes isTerminal report the given answers for stdin and stdout
// for the length of a test.
func useTerminals(t *testing.T, stdinIsTerminal, stdoutIsTerminal bool) {
	t.Helper()

	old := isTerminal
	t.Cleanup(func() { isTerminal = old })

	isTerminal = func(stream any) bool {
		switch stream {
		case stdin:
			return stdinIsTerminal
		case stdout:
			return stdoutIsTerminal
		}
		return false
	}
}

// writeFiles writes a file in a new directory for each of the contents, named
// as in names, and returns their paths.
func writeFiles(t *testing.T, names, contents []string) []string {
	t.Helper()

	dir := t.TempDir()

	var paths []string
	for i, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(contents[i]), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	return paths
}

func TestStdinToStdout(t *testing.T) {
	for _, args := range [][]string{{}, {"-"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, err := executeWithStdin(t, "hello\n", args)
			if err != nil {
				t.Fatal(err)
			}

			got, header := gunzip(t, out)
			if got != "hello\n" {
				t.Errorf("contents = %q, want %q", got, "hello\n")
			}

			// Compressed stdin has no name, and no mtime, which the reader
			// reports as a zero time.
			if header.Name != "" || !header.ModTime.IsZero() {
				t.Errorf("header = name %q, mtime %v; want none", header.Name, header.ModTime)
			}
		})
	}
}

func TestStdinToStdoutRsyncable(t *testing.T) {
	out, err := executeWithStdin(t, "hello\n", []string{"--rsyncable"})
	if err != nil {
		t.Fatal(err)
	}

	if got, _ := gunzip(t, out); got != "hello\n" {
		t.Errorf("contents = %q, want %q", got, "hello\n")
	}
}

func TestStdinDecompress(t *testing.T) {
	compressed, err := executeWithStdin(t, "some data\n", nil)
	if err != nil {
		t.Fatal(err)
	}

	out, err := executeWithStdin(t, compressed, []string{"-d"})
	if err != nil {
		t.Fatal(err)
	}

	if out != "some data\n" {
		t.Errorf("output = %q, want %q", out, "some data\n")
	}
}

func TestStdinTest(t *testing.T) {
	compressed, err := executeWithStdin(t, "some data\n", nil)
	if err != nil {
		t.Fatal(err)
	}

	out, err := executeWithStdin(t, compressed, []string{"-t"})
	if err != nil || out != "" {
		t.Errorf("good data: output = %q, error = %v; want neither", out, err)
	}

	_, err = executeWithStdin(t, "not gzip\n", []string{"-t"})
	if want := "klzip: error: stdin: error creating gzip reader"; err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("bad data: error = %v, want it to start with %q", err, want)
	}

	// A bad checksum is found by reading to the end.
	_, err = executeWithStdin(t, compressed[:len(compressed)-8]+"12345678", []string{"-t"})
	if err == nil {
		t.Error("bad checksum: expected an error")
	}
}

func TestStdinDecompressError(t *testing.T) {
	out, err := executeWithStdin(t, "not gzip\n", []string{"-d"})

	if want := "klzip: error: stdin: error creating gzip reader"; err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("error = %v, want it to start with %q", err, want)
	}
	if out != "" {
		t.Errorf("output = %q, want none", out)
	}
}

// Stdin that is a pipe, closed after the data, is read to its end.
func TestStdinFromPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, _ = w.WriteString("piped\n")
		_ = w.Close()
	}()

	out, errs := useStreams(t, r)

	cli, err := parseArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := cli.options()
	if err != nil {
		t.Fatal(err)
	}

	within(t, "the run", func() {
		if !Run(opts) {
			t.Errorf("failed: %s", errs.String())
		}
	})

	if got := mustGunzip(t, out.String()); got != "piped\n" {
		t.Errorf("contents = %q, want %q", got, "piped\n")
	}
}

func TestStdoutKeepsFile(t *testing.T) {
	path := writeFiles(t, []string{"a.txt"}, []string{"file data\n"})[0]

	out, err := executeWithStdin(t, "", []string{"-c", path})
	if err != nil {
		t.Fatal(err)
	}

	got, header := gunzip(t, out)
	if got != "file data\n" {
		t.Errorf("contents = %q, want %q", got, "file data\n")
	}
	if header.Name != "a.txt" {
		t.Errorf("name = %q, want %q", header.Name, "a.txt")
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("the file was not kept: %v", err)
	}
	if _, err := os.Stat(path + ".gz"); err == nil {
		t.Error("a .gz file was written")
	}

	// -n leaves the name out.
	out, err = executeWithStdin(t, "", []string{"-c", "-n", path})
	if err != nil {
		t.Fatal(err)
	}
	if _, header := gunzip(t, out); header.Name != "" {
		t.Errorf("name = %q, want none with -n", header.Name)
	}
}

// With -c, the output for several files comes out in the order of the files,
// whichever finishes first.
func TestStdoutSeveralFilesInOrder(t *testing.T) {
	contents := []string{
		strings.Repeat("first file\n", 400_000),
		"second file\n",
		"third file\n",
	}
	paths := writeFiles(t, []string{"a.txt", "b.txt", "c.txt"}, contents)

	var out string
	within(t, "the run", func() {
		var err error
		out, err = executeWithStdin(t, "", append([]string{"-c", "-j", "3"}, paths...))
		if err != nil {
			t.Error(err)
		}
	})

	if got, want := mustGunzip(t, out), strings.Join(contents, ""); got != want {
		t.Errorf("the output is not the files in order (%d bytes, want %d)", len(got), len(want))
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("a file was not kept: %v", err)
		}
	}
}

func TestStdoutDecompressSeveralFiles(t *testing.T) {
	compressed := func(contents string) string {
		out, err := executeWithStdin(t, contents, nil)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	paths := writeFiles(t, []string{"a.gz", "b.gz"}, []string{compressed("one\n"), compressed("two\n")})

	out, err := executeWithStdin(t, "", append([]string{"-c", "-d"}, paths...))
	if err != nil {
		t.Fatal(err)
	}

	if out != "one\ntwo\n" {
		t.Errorf("output = %q, want %q", out, "one\ntwo\n")
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("a file was not kept: %v", err)
		}
	}
}

func TestStdoutSkipsFilesWithSuffix(t *testing.T) {
	path := writeFiles(t, []string{"a.txt.gz"}, []string{"already compressed"})[0]

	out, errs := useStreams(t, strings.NewReader(""))

	cli, err := parseArgs([]string{"-c", path})
	if err != nil {
		t.Fatal(err)
	}
	opts, err := cli.options()
	if err != nil {
		t.Fatal(err)
	}

	// A skipped file is not a failure, and is reported.
	if !Run(opts) {
		t.Errorf("a skipped file was a failure: %s", errs.String())
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want none", out.String())
	}
	if want := "klzip: " + path + ": already has .gz suffix -- unchanged\n"; errs.String() != want {
		t.Errorf("stderr = %q, want %q", errs.String(), want)
	}
}

// A file that fails gives up its turn, so the files after it are still written.
func TestStdoutFailedFileDoesNotHoldUpOthers(t *testing.T) {
	paths := writeFiles(t, []string{"b.txt"}, []string{"second\n"})
	missing := filepath.Join(filepath.Dir(paths[0]), "missing.txt")

	var out string
	var err error
	within(t, "the run", func() {
		out, err = executeWithStdin(t, "", []string{"-c", "-j", "2", missing, paths[0]})
	})

	if want := "klzip: error: " + missing + ": "; err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("error = %v, want it to start with %q", err, want)
	}
	if got := mustGunzip(t, out); got != "second\n" {
		t.Errorf("output = %q, want the second file", got)
	}
}

// A file that has written some output and then fails also gives up its turn.
func TestStdoutFailedStreamDoesNotHoldUpOthers(t *testing.T) {
	// The first file is cut off in the middle of its data.  It holds more than
	// the output buffer, so some of its output is written before it fails.
	whole := gzipBytes(t, strings.Repeat("x\n", 200_000))
	cut := whole[:len(whole)/2]

	paths := writeFiles(t, []string{"a.gz", "b.gz"}, []string{string(cut), string(gzipBytes(t, "second\n"))})

	var out string
	var err error
	within(t, "the run", func() {
		out, err = executeWithStdin(t, "", []string{"-c", "-d", "-j", "2", paths[0], paths[1]})
	})

	if want := "klzip: error: " + paths[0] + ": "; err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("error = %v, want it to start with %q", err, want)
	}
	if !strings.HasPrefix(out, "x\n") {
		t.Errorf("the failed file wrote no output before it failed (%d bytes)", len(out))
	}
	if !strings.HasSuffix(out, "second\n") {
		t.Errorf("the second file's output is missing (%d bytes)", len(out))
	}
}

func TestTerminalRefusesToCompressToTerminal(t *testing.T) {
	useTerminals(t, false, true)

	_, err := executeWithStdin(t, "data\n", nil)
	if want := "klzip: error: stdin: compressed data not written to a terminal. Use -f to force compression."; err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("error = %v, want it to start with %q", err, want)
	}

	// -f forces it.
	out, err := executeWithStdin(t, "data\n", []string{"-f"})
	if err != nil {
		t.Fatal(err)
	}
	if got := mustGunzip(t, out); got != "data\n" {
		t.Errorf("contents = %q, want %q", got, "data\n")
	}
}

func TestTerminalRefusesToReadFromTerminal(t *testing.T) {
	useTerminals(t, true, false)

	for _, args := range [][]string{{"-d"}, {"-t"}} {
		_, err := executeWithStdin(t, "", args)
		if want := "klzip: error: stdin: compressed data not read from a terminal. Use -f to force decompression."; err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("%v: error = %v, want it to start with %q", args, err, want)
		}
	}

	// -f forces it, and then the data is read.
	compressed, err := executeWithStdin(t, "data\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := executeWithStdin(t, compressed, []string{"-d", "-f"}); err != nil || out != "data\n" {
		t.Errorf("with -f: output = %q, error = %v", out, err)
	}
}

// Only stdin is checked: files written to a terminal, and stdout that is a
// terminal for text, are allowed.
func TestTerminalAllowsOtherCases(t *testing.T) {
	useTerminals(t, true, true)

	path := writeFiles(t, []string{"a.txt"}, []string{"file data\n"})[0]

	// A named file to a terminal.
	out, err := executeWithStdin(t, "", []string{"-c", path})
	if err != nil {
		t.Fatal(err)
	}

	// Decompressing to a terminal, from a file.
	gzPath := writeFiles(t, []string{"a.gz"}, []string{out})[0]
	if out, err := executeWithStdin(t, "", []string{"-c", "-d", gzPath}); err != nil || out != "file data\n" {
		t.Errorf("decompress to a terminal: output = %q, error = %v", out, err)
	}

	// Files in place do not use the terminal at all.
	if err := executeArgs(t, []string{"-k", path}); err != nil {
		t.Error(err)
	}
}
