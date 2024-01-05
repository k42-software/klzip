# klzip

This is a command line tool for compressing and decompressing files. It is
intended to be a drop-in replacement for the standard `gzip` command.

For my use cases on my hardware, it is faster and produces smaller files. Your
mileage may vary.

## Installation

Grab the latest release from the github releases page, or build from source.

This project is compatible with the standard golang build tools.

## Usage

Usage is identical to the standard `gzip` command. Run `klzip -h` for more
information.

Files compressed with `klzip` can be decompressed with `gzip` and vice versa.

## rsyncable support using xflate

The file produced by `rsyncable` flag is constructed differently to that from
the standard `gzip` command. You may find that `rsync` will copy the entire
file on the first sync after switching to this tool. Subsequent syncs will be
faster.

Using the `rsyncable` flag has the side effect that the archive becomes seekable
when using a reader that supports the `xflate` format. This provides the ability
to read just a portion of the archive without decompressing the entire file.

See the following for more information on the `xflate` format:
https://github.com/dsnet/compress/blob/master/doc/xflate-format.pdf

## License

This project is covered by a BSD-style license that can be found in the LICENSE file.
