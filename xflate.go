// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/binary"
	"github.com/dsnet/compress/xflate"
	"github.com/dustin/go-humanize"
	"github.com/klauspost/compress/gzip"
	"github.com/pkg/errors"
	"hash/crc32"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	gzipID1       = 0x1f
	gzipID2       = 0x8b
	gzipDeflate   = 8
	gzipOSUnknown = 255
)

// XflateCompressFile is like CompressFile but writes the data as an xflate
// stream inside a gzip container, which makes the output seekable.
func XflateCompressFile(filename, suffix string, level int, keep, force, named, verbose bool) error {

	started := time.Now().UTC()

	if verbose {
		log.Printf(
			"Compressing %s (using xflate)",
			filename,
		)
	}

	res, err := convertFile(filename, keep, force, func(r io.Reader, info os.FileInfo) (*conversion, error) {
		return &conversion{
			outputFilename: filename + suffix,
			mtime:          info.ModTime(),
			run: func(w io.Writer) error {
				// Write Gzip header.
				header := &gzip.Header{
					Comment: "Encoded using XFLATE",
					OS:      gzipOSUnknown,
				}
				if named {
					header.Name = filepath.Base(filename)
					header.ModTime = info.ModTime()
				}
				if _, err := writeGzipHeader(w, header); err != nil {
					return errors.Wrap(err, "error writing header")
				}

				// Instead of using flate.Writer, we use xflate.Writer instead.
				// We choose a relative small chunk size of 64KiB for better
				// random access properties, at the expense of compression ratio.
				xw, err := xflate.NewWriter(w, &xflate.WriterConfig{
					Level:     level,
					ChunkSize: 1 << 16,
				})
				if err != nil {
					return errors.Wrap(err, "error creating xflate writer")
				}

				// Write the test data.
				crc := crc32.NewIEEE()
				mw := io.MultiWriter(xw, crc) // Write to both compressor and hasher
				written, err := io.Copy(mw, r)
				if err != nil {
					return errors.Wrap(err, "error writing data")
				}

				if err := xw.Flush(xflate.FlushSync); err != nil {
					return errors.Wrap(err, "error flushing data")
				}

				if err := xw.Close(); err != nil {
					return errors.Wrap(err, "error closing output writer")
				}

				// Write Gzip footer.
				if err := binary.Write(w, binary.LittleEndian, uint32(crc.Sum32())); err != nil {
					return errors.Wrap(err, "error writing footer: checksum")
				}
				if err := binary.Write(w, binary.LittleEndian, uint32(written)); err != nil {
					return errors.Wrap(err, "error writing footer: size")
				}

				return nil
			},
		}, nil
	})
	if err != nil {
		return err
	}

	if verbose {
		log.Printf(
			"Compressed %s from %s to %s (level %d) in %v",
			res.outputFilename,
			humanize.Bytes(uint64(res.inputSize)),
			humanize.Bytes(uint64(res.outputSize)),
			level,
			time.Since(started),
		)
	}

	return nil
}

func writeGzipHeader(w io.Writer, header *gzip.Header) (n int, err error) {

	// The Gzip header without using any extra features is 10 bytes long.
	var buffer [10]byte

	buffer[0] = gzipID1
	buffer[1] = gzipID2
	buffer[2] = gzipDeflate
	buffer[3] = 0
	if header.Extra != nil {
		buffer[3] |= 0x04
	}
	if header.Name != "" {
		buffer[3] |= 0x08
	}
	if header.Comment != "" {
		buffer[3] |= 0x10
	}
	if !header.ModTime.IsZero() {
		binary.LittleEndian.PutUint32(buffer[4:8], uint32(header.ModTime.Unix()))
	}
	buffer[8] = 0
	buffer[9] = header.OS

	n, err = w.Write(buffer[:10])
	if err != nil {
		return n, err
	}
	if header.Extra != nil {
		err = writeBytes(w, header.Extra)
		if err != nil {
			return n, err
		}
	}
	if header.Name != "" {
		err = writeString(w, header.Name)
		if err != nil {
			return n, err
		}
	}
	if header.Comment != "" {
		err = writeString(w, header.Comment)
		if err != nil {
			return n, err
		}
	}

	return n, nil
}

// writeBytes writes a length-prefixed byte slice to w.
func writeBytes(w io.Writer, b []byte) error {
	if len(b) > 0xffff {
		return errors.New("gzip.Write: Extra data is too large")
	}
	var buffer [2]byte
	binary.LittleEndian.PutUint16(buffer[:2], uint16(len(b)))
	_, err := w.Write(buffer[:2])
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// writeString writes a UTF-8 string s in GZIP's format to w.
// GZIP (RFC 1952) specifies that strings are NUL-terminated ISO 8859-1 (Latin-1).
func writeString(w io.Writer, s string) (err error) {
	// GZIP stores Latin-1 strings; error if non-Latin-1; convert if non-ASCII.
	needConversion := false
	for _, v := range s {
		if v == 0 || v > 0xff {
			return errors.New("gzip.Write: non-Latin-1 header string")
		}
		if v > 0x7f {
			needConversion = true
		}
	}
	if needConversion {
		b := make([]byte, 0, len(s))
		for _, v := range s {
			b = append(b, byte(v))
		}
		_, err = w.Write(b)
	} else {
		_, err = io.WriteString(w, s)
	}
	if err != nil {
		return err
	}
	// GZIP strings are NUL-terminated.
	_, err = w.Write([]byte{0})
	return err
}
