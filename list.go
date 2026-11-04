// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// The flags in the fourth byte of the header of a gzip stream.
const (
	gzipFlagHeaderCRC = 1 << 1
	gzipFlagExtra     = 1 << 2
	gzipFlagName      = 1 << 3
	gzipFlagComment   = 1 << 4
	gzipFlagReserved  = 0xe0
)

// gzipTrailerLength is the length of the trailer of a gzip stream: a checksum
// and the length of the data.
const gzipTrailerLength = 8

// listStdinName is the name shown for stdin when listing.
const listStdinName = "stdout"

// listVerbosePrefixWidth is the width of the columns that only the verbose
// listing has: the method, the checksum, the date and the time.
const listVerbosePrefixWidth = 27

// errNotGzip is the error for data that is not a gzip stream.
var errNotGzip = errors.New("not in gzip format")

// gzipHeader is what listing needs of the header of a gzip stream.
type gzipHeader struct {
	// mtime is the time stored in the header.  It is zero if none is stored.
	mtime time.Time

	// name is the name stored in the header.  It is empty if none is stored.
	name string

	// length is the number of bytes the header takes.
	length int64
}

// listing is what -l shows for one compressed file.  A size that is not known
// is -1.
type listing struct {
	// name and mtime are shown for the file.
	name  string
	mtime time.Time

	crc          uint32
	compressed   int64
	uncompressed int64

	// overhead is the part of the compressed size that is not compressed data:
	// the header and the trailer.
	overhead int64
}

// readGzipHeader reads the header of a gzip stream from r.  It returns
// errNotGzip if r does not start with one.
func readGzipHeader(r *bufio.Reader) (gzipHeader, error) {
	var header gzipHeader

	var fixed [10]byte
	if _, err := io.ReadFull(r, fixed[:]); err != nil {
		return header, errNotGzip
	}

	flags := fixed[3]
	if fixed[0] != 0x1f || fixed[1] != 0x8b || fixed[2] != 8 || flags&gzipFlagReserved != 0 {
		return header, errNotGzip
	}
	header.length = int64(len(fixed))

	if seconds := binary.LittleEndian.Uint32(fixed[4:8]); seconds != 0 {
		header.mtime = time.Unix(int64(seconds), 0)
	}

	if flags&gzipFlagExtra != 0 {
		var size [2]byte
		if _, err := io.ReadFull(r, size[:]); err != nil {
			return header, errNotGzip
		}

		extra := int64(binary.LittleEndian.Uint16(size[:]))
		if _, err := io.CopyN(io.Discard, r, extra); err != nil {
			return header, errNotGzip
		}
		header.length += int64(len(size)) + extra
	}

	if flags&gzipFlagName != 0 {
		name, err := r.ReadBytes(0)
		if err != nil {
			return header, errNotGzip
		}

		// The name is in Latin-1, which is the first 256 characters of Unicode.
		var latin1 strings.Builder
		for _, b := range name[:len(name)-1] {
			latin1.WriteRune(rune(b))
		}
		header.name = latin1.String()
		header.length += int64(len(name))
	}

	if flags&gzipFlagComment != 0 {
		comment, err := r.ReadBytes(0)
		if err != nil {
			return header, errNotGzip
		}
		header.length += int64(len(comment))
	}

	if flags&gzipFlagHeaderCRC != 0 {
		if _, err := io.CopyN(io.Discard, r, 2); err != nil {
			return header, errNotGzip
		}
		header.length += 2
	}

	return header, nil
}

// readListing reads what -l shows from the gzip file in r, which is size bytes
// long.  The name is the name of the file, and the modification time is the
// time of the file.  With named, the name and the time stored in the file are
// used when it has them, and a stored name is shown in dir.
func readListing(r io.ReadSeeker, size int64, name string, mtime time.Time, named bool, dir string) (*listing, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, errors.Wrap(err, "error reading input file")
	}

	header, err := readGzipHeader(bufio.NewReader(r))
	if err != nil {
		return nil, err
	}

	if size < header.length+gzipTrailerLength {
		return nil, errNotGzip
	}

	var trailer [gzipTrailerLength]byte
	if _, err := r.Seek(size-gzipTrailerLength, io.SeekStart); err != nil {
		return nil, errors.Wrap(err, "error reading input file")
	}
	if _, err := io.ReadFull(r, trailer[:]); err != nil {
		return nil, errors.Wrap(err, "error reading input file")
	}

	if named {
		if header.name != "" {
			name = filepath.Join(dir, header.name)
		}
		if !header.mtime.IsZero() {
			mtime = header.mtime
		}
	}

	return &listing{
		name:         name,
		mtime:        mtime,
		crc:          binary.LittleEndian.Uint32(trailer[0:4]),
		compressed:   size,
		uncompressed: int64(binary.LittleEndian.Uint32(trailer[4:8])),
		overhead:     header.length + gzipTrailerLength,
	}, nil
}

// listFile reads what -l shows for a file, or for stdin if file is stdinName.
func listFile(opts Options, file string) (*listing, error) {
	if file == stdinName {
		return listStdin(opts)
	}

	input, err := os.Open(file)
	if err != nil {
		return nil, errors.Wrap(err, "error opening input file")
	}
	//goland:noinspection GoUnhandledErrorResult
	defer input.Close()

	info, err := input.Stat()
	if err != nil {
		return nil, errors.Wrap(err, "error stating input file")
	}

	name := file
	if stripped, ok := decompressedName(file, opts.Suffix); ok {
		name = stripped
	}

	return readListing(input, info.Size(), name, info.ModTime(), opts.Named, filepath.Dir(file))
}

// listStdin reads what -l shows for stdin.  The sizes are only known when stdin
// can be sought, as when it is a file, and are -1 when it is a pipe.
func listStdin(opts Options) (*listing, error) {
	if err := checkTerminals(opts); err != nil {
		return nil, err
	}

	unknown := &listing{name: listStdinName, compressed: -1, uncompressed: -1}

	seeker, ok := stdin.(io.ReadSeeker)
	if !ok {
		return unknown, nil
	}

	size, err := seeker.Seek(0, io.SeekEnd)
	if err != nil {
		return unknown, nil
	}

	// The time of the file is shown when stdin is one.
	var mtime time.Time
	if file, isFile := stdin.(*os.File); isFile {
		if info, err := file.Stat(); err == nil {
			mtime = info.ModTime()
		}
	}

	return readListing(seeker, size, listStdinName, mtime, opts.Named, ".")
}

// runList writes the listing of the files to stdout, in the order of the files.
// What could not be listed is written to stderr.  It reports whether every file
// was listed.
func runList(opts Options) bool {
	index := make(map[string]int, len(opts.Files))
	for i, file := range opts.Files {
		index[file] = i
	}

	listings := make([]*listing, len(opts.Files))
	ok := forEachFile(opts.Files, opts.Jobs, stderr, func(file string, _ *log.Logger) error {
		l, err := listFile(opts, file)
		listings[index[file]] = l
		return err
	})

	var listed []*listing
	for _, l := range listings {
		if l != nil {
			listed = append(listed, l)
		}
	}
	if len(listed) == 0 {
		return ok
	}

	// Quiet leaves out the heading and the totals, and turns verbose off.
	verbose := opts.Verbose && !opts.Quiet

	out := bufio.NewWriter(stdout)
	if !opts.Quiet {
		_, _ = fmt.Fprintln(out, formatListHeader(verbose))
	}
	for _, l := range listed {
		_, _ = fmt.Fprintln(out, formatListLine(l, verbose))
	}
	if len(opts.Files) > 1 && !opts.Quiet {
		_, _ = fmt.Fprintln(out, formatListTotals(listed, verbose))
	}
	_ = out.Flush()

	return ok
}

// listRatio returns the compression ratio the way gzip shows it: as a
// percentage with one decimal, with a sign column in front.  It leaves out the
// bytes that are not compressed data, which are the overhead.
func listRatio(compressed, uncompressed, overhead int64) string {
	var percent float64
	if compressed >= 0 && uncompressed > 0 {
		saved := uncompressed - (compressed - overhead)
		percent = 100 * float64(saved) / float64(uncompressed)
	}

	sign := " "
	if percent < 0 {
		sign = "-"
		percent = -percent
	}

	return fmt.Sprintf("%s%4.1f%%", sign, percent)
}

// formatListHeader returns the heading line of the listing.
func formatListHeader(verbose bool) string {
	line := fmt.Sprintf("%19s %19s %s %s", "compressed", "uncompressed", " ratio", "uncompressed_name")
	if verbose {
		return fmt.Sprintf("%-*s %s", listVerbosePrefixWidth, "method  crc     date  time", line)
	}

	return line
}

// formatListLine returns the line of the listing for one file.
func formatListLine(l *listing, verbose bool) string {
	line := fmt.Sprintf("%19d %19d %s %s",
		l.compressed, l.uncompressed, listRatio(l.compressed, l.uncompressed, l.overhead), l.name)
	if verbose {
		date := l.mtime.Local().Format("Jan _2 15:04")
		if l.mtime.IsZero() {
			date = time.Unix(0, 0).Local().Format("Jan _2 15:04")
		}
		// The checksum is all ones when the sizes are not known.
		crc := fmt.Sprintf("%08x", l.crc)
		if l.compressed < 0 {
			crc = "ffffffffffffffff"
		}

		return fmt.Sprintf("defla %s %s %s", crc, date, line)
	}

	return line
}

// formatListTotals returns the last line of the listing, for several files.
func formatListTotals(listings []*listing, verbose bool) string {
	var total listing
	for _, l := range listings {
		if l.compressed < 0 {
			continue
		}
		total.compressed += l.compressed
		total.uncompressed += l.uncompressed
		total.overhead += l.overhead
	}
	total.name = "(totals)"

	line := formatListLine(&total, false)
	if verbose {
		return strings.Repeat(" ", listVerbosePrefixWidth) + " " + line
	}

	return line
}
