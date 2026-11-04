// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"path/filepath"
	"strings"
)

// suffixRule says that a file name ending in suffix is a compressed file, and
// what takes the place of the suffix in its decompressed name.
type suffixRule struct {
	suffix      string
	replacement string
}

// suffixRules are the suffixes of compressed files.  The last two are tar
// archives.  Names are matched ignoring case, so ".z" also matches ".Z".  A
// suffix says nothing about the format of the file: klzip reads only gzip
// streams, and fails on a file in another format, such as one made by compress
// (.Z and .taz) or pack (.z).
var suffixRules = []suffixRule{
	{".gz", ""},
	{"-gz", ""},
	{".z", ""},
	{"-z", ""},
	{"_z", ""},
	{".tgz", ".tar"},
	{".taz", ".tar"},
}

// matchSuffix returns the rule for the suffix that filename ends in, and
// whether it ends in one.  The suffix of the rule is the text that filename
// ends in, in its own case.  The extra suffix is a rule too, with nothing in its
// place, and is looked at first.
func matchSuffix(filename, extra string) (suffixRule, bool) {
	if extra != "" && hasSuffixFold(filename, extra) {
		return suffixRule{suffix: filename[len(filename)-len(extra):]}, true
	}

	for _, rule := range suffixRules {
		if hasSuffixFold(filename, rule.suffix) {
			return suffixRule{
				suffix:      filename[len(filename)-len(rule.suffix):],
				replacement: rule.replacement,
			}, true
		}
	}

	return suffixRule{}, false
}

// hasSuffixFold reports whether s ends in suffix, ignoring case.
func hasSuffixFold(s, suffix string) bool {
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

// decompressedName returns the name filename has once decompressed, and whether
// it ends in a suffix that says it is compressed.  A name that is only the
// suffix, such as ".gz" or "dir/.gz", has nothing left to name the result.
func decompressedName(filename, extra string) (string, bool) {
	rule, ok := matchSuffix(filename, extra)
	if !ok {
		return "", false
	}

	stem := filename[:len(filename)-len(rule.suffix)]
	if stem == "" || strings.HasSuffix(stem, string(filepath.Separator)) {
		return "", false
	}

	return stem + rule.replacement, true
}
