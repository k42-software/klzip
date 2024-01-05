// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"encoding/binary"
	"github.com/dsnet/compress/xflate"
	"github.com/dustin/go-humanize"
	"github.com/google/renameio"
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

func XflateCompressFile(filename, suffix string, level int, keep, force, named, verbose bool) (err error) {

	var (
		inputInfo  os.FileInfo
		outputInfo os.FileInfo
		inputFile  *os.File
		outputFile *renameio.PendingFile
		xw         *xflate.Writer
		written    int64

		outputFilename string
	)

	started := time.Now().UTC()

	if verbose {
		log.Printf(
			"Compressing %s (using xflate)",
			filename,
		)
	}

	inputInfo, err = os.Stat(filename)
	if err != nil {
		return errors.Wrap(err, "error stating input file")
	}

	inputFile, err = os.Open(filename)
	if err != nil {
		return errors.Wrap(err, "error opening input file")
	}
	//goland:noinspection GoUnhandledErrorResult
	defer inputFile.Close()

	outputFilename = filename + suffix

	if !force {
		if _, err := os.Stat(outputFilename); err == nil {
			return errors.New("output file already exists")
		}
	}

	outputFile, err = renameio.TempFile(filepath.Dir(outputFilename), outputFilename)
	if err != nil {
		return errors.Wrap(err, "error creating temporary output file")
	}
	//goland:noinspection GoUnhandledErrorResult
	defer outputFile.Cleanup()

	outputBuffer := bufio.NewWriter(outputFile)

	// Write Gzip header.
	header := &gzip.Header{
		Comment: "Encoded using XFLATE",
		OS:      gzipOSUnknown,
	}
	if named {
		header.Name = filepath.Base(filename)
		header.ModTime = inputInfo.ModTime()
	}
	_, err = writeGzipHeader(outputBuffer, header)
	if err != nil {
		return errors.Wrap(err, "error writing header")
	}

	// Instead of using flate.Writer, we use xflate.Writer instead.
	// We choose a relative small chunk size of 64KiB for better
	// random access properties, at the expense of compression ratio.
	xw, err = xflate.NewWriter(outputBuffer, &xflate.WriterConfig{
		Level:     level,
		ChunkSize: 1 << 16,
	})
	if err != nil {
		return errors.Wrap(err, "error creating xflate writer")
	}

	// Write the test data.
	crc := crc32.NewIEEE()
	mw := io.MultiWriter(xw, crc) // Write to both compressor and hasher
	written, err = io.Copy(mw, bufio.NewReader(inputFile))
	if err != nil {
		return errors.Wrap(err, "error writing data")
	}

	if err := inputFile.Close(); err != nil {
		return errors.Wrap(err, "error closing input file")
	}

	if err := xw.Flush(xflate.FlushSync); err != nil {
		return errors.Wrap(err, "error flushing data")
	}

	if err := xw.Close(); err != nil {
		return errors.Wrap(err, "error closing output writer")
	}

	// Write Gzip footer.
	err = binary.Write(outputBuffer, binary.LittleEndian, uint32(crc.Sum32()))
	if err != nil {
		return errors.Wrap(err, "error writing footer: checksum")
	}
	err = binary.Write(outputBuffer, binary.LittleEndian, uint32(written))
	if err != nil {
		return errors.Wrap(err, "error writing footer: size")
	}

	if err := outputBuffer.Flush(); err != nil {
		return errors.Wrap(err, "error flushing output buffer")
	}

	if err := outputFile.CloseAtomicallyReplace(); err != nil {
		return errors.Wrap(err, "error atomically closing output file")
	}

	outputInfo, err = os.Stat(outputFilename)
	if err != nil {
		return errors.Wrap(err, "error stating output file")
	}

	if !keep {
		if err = os.Remove(filename); err != nil {
			return errors.Wrap(err, "error removing input file")
		}
	}

	if verbose {
		duration := time.Since(started)
		log.Printf(
			"Compressed %s from %s to %s (level %d) in %v",
			outputFilename,
			humanize.Bytes(uint64(inputInfo.Size())),
			humanize.Bytes(uint64(outputInfo.Size())),
			level,
			duration,
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
	binary.LittleEndian.PutUint32(buffer[4:8], uint32(header.ModTime.Unix()))
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
