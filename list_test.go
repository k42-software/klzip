// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	stdgzip "compress/gzip"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gzipFile returns a gzip stream whose header has the given fields.  Its data is
// not compressed, so the size of the stream is known without running a
// compressor.
func gzipFile(name string, mtime uint32, extra, comment string, headerCRC bool, data string) []byte {
	var flags byte
	if extra != "" {
		flags |= gzipFlagExtra
	}
	if name != "" {
		flags |= gzipFlagName
	}
	if comment != "" {
		flags |= gzipFlagComment
	}
	if headerCRC {
		flags |= gzipFlagHeaderCRC
	}

	out := []byte{0x1f, 0x8b, 8, flags, byte(mtime), byte(mtime >> 8), byte(mtime >> 16), byte(mtime >> 24), 0, 3}
	if extra != "" {
		out = append(out, byte(len(extra)), byte(len(extra)>>8))
		out = append(out, extra...)
	}
	if name != "" {
		out = append(out, name...)
		out = append(out, 0)
	}
	if comment != "" {
		out = append(out, comment...)
		out = append(out, 0)
	}
	if headerCRC {
		out = append(out, 0, 0)
	}

	// A stored deflate block holding the data.
	n := len(data)
	out = append(out, 1, byte(n), byte(n>>8), byte(^n), byte(^n>>8))
	out = append(out, data...)

	sum := crc32.ChecksumIEEE([]byte(data))
	return append(out, byte(sum), byte(sum>>8), byte(sum>>16), byte(sum>>24), byte(n), byte(n>>8), byte(n>>16), byte(n>>24))
}

// listFiles lists the files with the given command line, and returns what is
// written to stdout, and the error that main would exit with.
func listFiles(t *testing.T, args ...string) (string, error) {
	t.Helper()

	return executeWithStdin(t, "", append([]string{"-l"}, args...))
}

func TestReadGzipHeader(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantName   string
		wantMtime  int64
		wantLength int64
	}{
		{"fixed fields only", gzipFile("", 0, "", "", false, "x"), "", 0, 10},
		{"name", gzipFile("a.txt", 0, "", "", false, "x"), "a.txt", 0, 16},
		{"mtime", gzipFile("", 1583298367, "", "", false, "x"), "", 1583298367, 10},
		{"extra", gzipFile("", 0, "abc", "", false, "x"), "", 0, 15},
		{"comment", gzipFile("", 0, "", "hello", false, "x"), "", 0, 16},
		{"header checksum", gzipFile("", 0, "", "", true, "x"), "", 0, 12},
		{"everything", gzipFile("a.txt", 5, "abc", "hello", true, "x"), "a.txt", 5, 10 + 5 + 6 + 6 + 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header, err := readGzipHeader(bufio.NewReader(bytes.NewReader(tt.data)))
			if err != nil {
				t.Fatal(err)
			}

			if header.name != tt.wantName {
				t.Errorf("name = %q, want %q", header.name, tt.wantName)
			}
			if got := header.mtime.Unix(); tt.wantMtime == 0 && !header.mtime.IsZero() || tt.wantMtime != 0 && got != tt.wantMtime {
				t.Errorf("mtime = %v, want %d", header.mtime, tt.wantMtime)
			}
			if header.length != tt.wantLength {
				t.Errorf("length = %d, want %d", header.length, tt.wantLength)
			}
		})
	}
}

func TestReadGzipHeaderNotGzip(t *testing.T) {
	good := gzipFile("a.txt", 0, "abc", "hello", true, "x")

	tests := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"text", []byte("this is not gzip")},
		{"short", good[:6]},
		{"wrong method", append([]byte{0x1f, 0x8b, 7}, good[3:]...)},
		{"reserved flags", append([]byte{0x1f, 0x8b, 8, 0x20}, good[4:]...)},
		{"cut off in the extra field", good[:12]},
		{"cut off in the name", good[:17]},
		{"cut off in the comment", good[:24]},
		{"cut off in the header checksum", good[:26]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := readGzipHeader(bufio.NewReader(bytes.NewReader(tt.data))); err != errNotGzip {
				t.Errorf("error = %v, want %v", err, errNotGzip)
			}
		})
	}
}

func TestReadListing(t *testing.T) {
	data := "some data to list"
	file := gzipFile("stored.txt", 1583298367, "", "", false, data)
	fileTime := time.Date(2021, 12, 25, 10, 11, 12, 0, time.UTC)

	read := func(name string, named bool, dir string) *listing {
		t.Helper()

		l, err := readListing(bytes.NewReader(file), int64(len(file)), name, fileTime, named, dir)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}

	l := read("file.txt", false, ".")
	if l.name != "file.txt" || !l.mtime.Equal(fileTime) {
		t.Errorf("without named: name = %q, mtime = %v; want the file's", l.name, l.mtime)
	}
	if l.compressed != int64(len(file)) || l.uncompressed != int64(len(data)) {
		t.Errorf("sizes = %d, %d; want %d, %d", l.compressed, l.uncompressed, len(file), len(data))
	}
	if want := crc32.ChecksumIEEE([]byte(data)); l.crc != want {
		t.Errorf("crc = %08x, want %08x", l.crc, want)
	}
	if want := int64(10 + len("stored.txt") + 1 + 8); l.overhead != want {
		t.Errorf("overhead = %d, want %d", l.overhead, want)
	}

	// With named, the stored name and time are shown, and the name goes in the
	// directory of the file.
	l = read("file.txt", true, "dir")
	if l.name != filepath.Join("dir", "stored.txt") || l.mtime.Unix() != 1583298367 {
		t.Errorf("with named: name = %q, mtime = %v; want the stored ones", l.name, l.mtime)
	}

	// Without a stored name or time, named shows the file's.
	bare := gzipFile("", 0, "", "", false, data)
	l, err := readListing(bytes.NewReader(bare), int64(len(bare)), "file.txt", fileTime, true, ".")
	if err != nil {
		t.Fatal(err)
	}
	if l.name != "file.txt" || !l.mtime.Equal(fileTime) {
		t.Errorf("nothing stored: name = %q, mtime = %v; want the file's", l.name, l.mtime)
	}
}

func TestReadListingNotGzip(t *testing.T) {
	good := gzipFile("a.txt", 0, "", "", false, "data")

	tests := []struct {
		name string
		data []byte
	}{
		{"text", []byte("this is not gzip data at all")},
		{"no room for the trailer", good[:20]},
		{"only a header", good[:16]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readListing(bytes.NewReader(tt.data), int64(len(tt.data)), "f", time.Time{}, false, ".")
			if err != errNotGzip {
				t.Errorf("error = %v, want %v", err, errNotGzip)
			}
		})
	}
}

// The ratios are the ones gzip shows.
func TestListRatio(t *testing.T) {
	tests := []struct {
		name         string
		compressed   int64
		uncompressed int64
		overhead     int64
		want         string
	}{
		// The ratio is 52.99%, which rounds up and does not truncate.
		{"a file", 1854, 3893, 24, " 53.0%"},
		{"a big file", 109150, 288894, 24, " 62.2%"},
		{"an empty file", 30, 0, 28, "  0.0%"},
		{"larger than the original", 32, 4, 26, "-50.0%"},
		{"much larger than the original", 22, 2, 18, "-100.0%"},
		{"sizes not known", -1, -1, 0, "  0.0%"},
		{"a single digit", 90, 100, 10, " 20.0%"},
		{"everything saved", 18, 100, 18, " 100.0%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := listRatio(tt.compressed, tt.uncompressed, tt.overhead); got != tt.want {
				t.Errorf("listRatio(%d, %d, %d) = %q, want %q",
					tt.compressed, tt.uncompressed, tt.overhead, got, tt.want)
			}
		})
	}
}

// The lines are what gzip prints.
func TestFormatList(t *testing.T) {
	mtime := time.Date(2020, 11, 4, 15, 25, 0, 0, time.Local)

	a := &listing{name: "a.txt", mtime: mtime, crc: 0x8dc4565d, compressed: 1854, uncompressed: 3893, overhead: 24}
	b := &listing{name: "b.txt", mtime: mtime, crc: 0xfb23b145, compressed: 109150, uncompressed: 288894, overhead: 24}
	empty := &listing{name: "empty.txt", mtime: mtime, compressed: 30, uncompressed: 0, overhead: 28}
	pipe := &listing{name: "stdout", compressed: -1, uncompressed: -1}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"header", formatListHeader(false),
			"         compressed        uncompressed  ratio uncompressed_name"},
		{"verbose header", formatListHeader(true),
			"method  crc     date  time           compressed        uncompressed  ratio uncompressed_name"},
		{"line", formatListLine(a, false),
			"               1854                3893  53.0% a.txt"},
		{"verbose line", formatListLine(a, true),
			"defla 8dc4565d Nov  4 15:25                1854                3893  53.0% a.txt"},
		{"big line", formatListLine(b, false),
			"             109150              288894  62.2% b.txt"},
		{"empty file", formatListLine(empty, false),
			"                 30                   0   0.0% empty.txt"},
		{"sizes not known", formatListLine(pipe, false),
			"                 -1                  -1   0.0% stdout"},
		{"verbose line with sizes not known", formatListLine(pipe, true),
			"defla ffffffffffffffff " + time.Unix(0, 0).Local().Format("Jan _2 15:04") +
				"                  -1                  -1   0.0% stdout"},
		{"totals", formatListTotals([]*listing{a, b}, false),
			"             111004              292787  62.1% (totals)"},
		{"verbose totals", formatListTotals([]*listing{a, b}, true),
			"                                         111004              292787  62.1% (totals)"},
		{"totals leave out sizes not known", formatListTotals([]*listing{a, pipe}, false),
			"               1854                3893  53.0% (totals)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("\n got %q\nwant %q", tt.got, tt.want)
			}
		})
	}
}

func TestListFiles(t *testing.T) {
	a := string(gzipFile("", 0, "", "", false, strings.Repeat("a", 100)))
	b := string(gzipFile("", 0, "", "", false, strings.Repeat("b", 2000)))
	paths := writeFiles(t, []string{"a.gz", "b.gz"}, []string{a, b})

	out, err := listFiles(t, paths[0], paths[1])
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("%d lines, want a heading, two files and totals:\n%s", len(lines), out)
	}
	if lines[0] != formatListHeader(false) {
		t.Errorf("heading = %q", lines[0])
	}
	if !strings.HasSuffix(lines[1], " "+filepath.Join(filepath.Dir(paths[0]), "a")) ||
		!strings.HasSuffix(lines[2], " "+filepath.Join(filepath.Dir(paths[1]), "b")) {
		t.Errorf("the files are not in order:\n%s", out)
	}
	if !strings.HasSuffix(lines[3], " (totals)") {
		t.Errorf("last line = %q, want the totals", lines[3])
	}
}

func TestListFilesQuiet(t *testing.T) {
	paths := writeFiles(t, []string{"a.gz", "b.gz"}, []string{
		string(gzipFile("", 0, "", "", false, "one")),
		string(gzipFile("", 0, "", "", false, "two")),
	})

	// Quiet leaves out the heading and the totals, and turns verbose off.
	out, err := listFiles(t, "-q", "-v", paths[0], paths[1])
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("%d lines, want one for each file:\n%s", len(lines), out)
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "defla") {
			t.Errorf("a verbose line with -q: %q", line)
		}
	}
}

func TestListFilesVerbose(t *testing.T) {
	path := writeFiles(t, []string{"a.gz"}, []string{string(gzipFile("", 0, "", "", false, "data"))})[0]

	out, err := listFiles(t, "-v", path)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "method  crc     date  time") ||
		!strings.HasPrefix(lines[1], "defla ") {
		t.Errorf("not a verbose listing:\n%s", out)
	}
}

// With -N the stored name is shown, in the directory of the file.
func TestListFilesNamed(t *testing.T) {
	path := writeFiles(t, []string{"renamed.gz"}, []string{string(gzipFile("stored.txt", 0, "", "", false, "data"))})[0]
	dir := filepath.Dir(path)

	out, err := listFiles(t, path)
	if err != nil || !strings.HasSuffix(strings.TrimSpace(out), " "+filepath.Join(dir, "renamed")) {
		t.Errorf("without -N: output = %q, error = %v; want the name of the file", out, err)
	}

	out, err = listFiles(t, "-N", path)
	if err != nil || !strings.HasSuffix(strings.TrimSpace(out), " "+filepath.Join(dir, "stored.txt")) {
		t.Errorf("with -N: output = %q, error = %v; want the stored name", out, err)
	}
}

// A file that cannot be listed is reported, and the others are still listed.
func TestListFilesFailures(t *testing.T) {
	paths := writeFiles(t, []string{"good.gz", "bad.gz"}, []string{
		string(gzipFile("", 0, "", "", false, "data")),
		"this is not gzip data at all",
	})
	missing := filepath.Join(filepath.Dir(paths[0]), "missing.gz")

	out, err := listFiles(t, paths[1], paths[0], missing)

	if err == nil || !strings.Contains(err.Error(), "klzip: error: "+paths[1]+": not in gzip format") ||
		!strings.Contains(err.Error(), "klzip: error: "+missing+": ") {
		t.Errorf("error = %v, want both failures", err)
	}
	if !strings.Contains(out, "good") || !strings.HasPrefix(out, formatListHeader(false)) {
		t.Errorf("output = %q, want the good file listed", out)
	}

	// Nothing listed means no heading.
	out, _ = listFiles(t, paths[1])
	if out != "" {
		t.Errorf("output = %q, want none when nothing is listed", out)
	}
}

// The file is listed as it is made by klzip, as well as by the writer here.
func TestListFilesFromKlzip(t *testing.T) {
	path := writeFiles(t, []string{"a.txt"}, []string{strings.Repeat("hello\n", 1000)})[0]

	if err := executeArgs(t, []string{path}); err != nil {
		t.Fatal(err)
	}

	out, err := listFiles(t, path+".gz")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 || !strings.Contains(lines[1], " 6000 ") || !strings.HasSuffix(lines[1], " "+path) {
		t.Errorf("output = %q, want the 6000 byte file listed under its own name", out)
	}
}

func TestListStdin(t *testing.T) {
	data := gzipFile("", 0, "", "", false, strings.Repeat("x", 500))

	// A file that can be sought gives the sizes.
	input := filepath.Join(t.TempDir(), "in.gz")
	if err := os.WriteFile(input, data, 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	out, errs := useStreams(t, file)
	_ = errs
	cli, err := parseArgs([]string{"-l"})
	if err != nil {
		t.Fatal(err)
	}
	opts, err := cli.options()
	if err != nil {
		t.Fatal(err)
	}
	if !Run(opts) {
		t.Fatalf("failed: %s", errs.String())
	}

	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[1], " stdout") || !strings.Contains(lines[1], " 500 ") {
		t.Errorf("output = %q, want the sizes of the file, named stdout", out.String())
	}
}

// Stdin that is a file shows the time of the file.  With -N it shows the name and
// time stored in it.
func TestListStdinFileTimes(t *testing.T) {
	stored := time.Date(2018, 6, 7, 8, 9, 10, 0, time.Local)
	fileTime := time.Date(2021, 12, 25, 10, 11, 12, 0, time.Local)

	path := filepath.Join(t.TempDir(), "in.gz")
	if err := os.WriteFile(path, gzipFile("stored.txt", uint32(stored.Unix()), "", "", false, "some data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fileTime, fileTime); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	list := func(args ...string) string {
		t.Helper()

		out, errs := useStreams(t, file)
		cli, err := parseArgs(append([]string{"-l", "-v"}, args...))
		if err != nil {
			t.Fatal(err)
		}
		opts, err := cli.options()
		if err != nil {
			t.Fatal(err)
		}
		if !Run(opts) {
			t.Fatalf("failed: %s", errs.String())
		}
		return out.String()
	}

	const layout = "Jan _2 15:04"

	out := list()
	if !strings.Contains(out, fileTime.Format(layout)) || !strings.HasSuffix(strings.TrimSpace(out), " stdout") {
		t.Errorf("without -N: output = %q, want the time of the file and the name stdout", out)
	}

	out = list("-N")
	if !strings.Contains(out, stored.Format(layout)) || !strings.HasSuffix(strings.TrimSpace(out), " stored.txt") {
		t.Errorf("with -N: output = %q, want the stored time and name", out)
	}
}

func TestListStdinPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	go func() {
		_, _ = w.Write(gzipFile("", 0, "", "", false, "piped"))
		_ = w.Close()
	}()

	out, _ := useStreams(t, r)

	cli, err := parseArgs([]string{"-l"})
	if err != nil {
		t.Fatal(err)
	}
	opts, err := cli.options()
	if err != nil {
		t.Fatal(err)
	}

	within(t, "the listing", func() { Run(opts) })

	// The sizes cannot be found without seeking, as in gzip.
	want := formatListLine(&listing{name: "stdout", compressed: -1, uncompressed: -1}, false)
	if !strings.Contains(out.String(), want) {
		t.Errorf("output = %q, want the line %q", out.String(), want)
	}
}

func TestListStdinTerminal(t *testing.T) {
	useTerminals(t, true, false)

	_, err := executeWithStdin(t, "", []string{"-l"})
	if want := "klzip: error: stdin: compressed data not read from a terminal."; err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("error = %v, want it to start with %q", err, want)
	}
}

// The listing agrees with the file made by the standard library, which stores
// the data with a header of its own.
func TestListFilesStdlibGzip(t *testing.T) {
	var buf bytes.Buffer
	zw := stdgzip.NewWriter(&buf)
	zw.Name = "stdlib.txt"
	if _, err := io.WriteString(zw, strings.Repeat("z", 3000)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	path := writeFiles(t, []string{"s.gz"}, []string{buf.String()})[0]

	out, err := listFiles(t, "-N", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, " 3000 ") || !strings.Contains(out, "stdlib.txt") {
		t.Errorf("output = %q, want 3000 bytes under the stored name", out)
	}
}
