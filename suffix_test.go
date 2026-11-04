// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestMatchSuffix(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		extra     string
		want      string
		wantMatch bool
	}{
		{"gz", "a.txt.gz", "", ".gz", true},
		{"hyphen gz", "a-gz", "", "-gz", true},
		{"z", "a.z", "", ".z", true},
		{"hyphen z", "a-z", "", "-z", true},
		{"underscore z", "a_z", "", "_z", true},
		{"tgz", "a.tgz", "", ".tgz", true},
		{"no suffix", "a", "", "", false},
		{"other suffix", "a.foo", "", "", false},
		{"upper case Z", "a.Z", "", ".Z", true},
		{"taz", "a.taz", "", ".taz", true},
		{"suffix in the middle", "a.gz.txt", "", "", false},

		// The suffix returned is the text in the file name, in its own case.
		{"upper case gz", "A.GZ", "", ".GZ", true},
		{"mixed case tgz", "a.Tgz", "", ".Tgz", true},
		{"upper case hyphen gz", "a-GZ", "", "-GZ", true},
		{"upper case taz", "a.TAZ", "", ".TAZ", true},
		{"extra suffix in another case", "a.FOO", ".foo", ".FOO", true},
		{"file name shorter than a suffix", "z", "", "", false},

		{"extra suffix", "a.foo", ".foo", ".foo", true},
		{"extra suffix keeps the list", "a.tgz", ".foo", ".tgz", true},
		{"extra suffix is looked at first", "a.tar.gz", ".tar.gz", ".tar.gz", true},
		{"extra suffix not matched", "a.bar", ".foo", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, ok := matchSuffix(tt.filename, tt.extra)
			if ok != tt.wantMatch || rule.suffix != tt.want {
				t.Errorf("matchSuffix(%q, %q) = %q, %v; want %q, %v",
					tt.filename, tt.extra, rule.suffix, ok, tt.want, tt.wantMatch)
			}
		})
	}
}

func TestDecompressedName(t *testing.T) {
	tests := []struct {
		filename string
		extra    string
		want     string
		wantOK   bool
	}{
		{"a.txt.gz", "", "a.txt", true},
		{"a-gz", "", "a", true},
		{"a.z", "", "a", true},
		{"a-z", "", "a", true},
		{"a_z", "", "a", true},
		{"a.tgz", "", "a.tar", true},
		{"dir/a.tgz", "", "dir/a.tar", true},
		{"a.foo", ".foo", "a", true},
		{"a.tgz", ".foo", "a.tar", true},
		{"a", "", "", false},
		{"a.foo", "", "", false},
		{"a.Z", "", "a", true},
		{"a.taz", "", "a.tar", true},

		// Case is ignored, and .tar is always lower case.
		{"UPPER.GZ", "", "UPPER", true},
		{"Mixed.Gz", "", "Mixed", true},
		{"FOO.TGZ", "", "FOO.tar", true},
		{"foo.Tgz", "", "foo.tar", true},
		{"bar.TAZ", "", "bar.tar", true},
		{"baz-GZ", "", "baz", true},
		{"qux_Z", "", "qux", true},
		{"a.FOO", ".foo", "a", true},

		// Nothing is left to name the result.
		{".gz", "", "", false},
		{"dir/.gz", "", "", false},
		{".tgz", "", "", false},
		{".foo", ".foo", "", false},
		{"dir/.tgz", "", "", false},
		{"dir/.a.gz", "", "dir/.a", true},
	}

	for _, tt := range tests {
		got, ok := decompressedName(tt.filename, tt.extra)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("decompressedName(%q, %q) = %q, %v; want %q, %v",
				tt.filename, tt.extra, got, ok, tt.want, tt.wantOK)
		}
	}
}
