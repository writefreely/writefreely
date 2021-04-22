/*
 * Copyright © 2018-2020 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

// Package parse assists in the parsing of plain text posts
package parse

import (
	"github.com/writeas/web-core/stringmanip"
	"regexp"
	"strings"
)

var (
	titleElementReg = regexp.MustCompile("</?p>")
	urlReg          = regexp.MustCompile("https?://")
	imgReg          = regexp.MustCompile(`!\[([^]]+)\]\([^)]+\)`)
)

// PostLede attempts to extract the first thought of the given post, generally
// contained within the first line or sentence of text.
func PostLede(t string, includePunc bool) string {
	// Adjust where we truncate if we want to include punctuation
	iAdj := 0
	if includePunc {
		iAdj = 1
	}

	// Find lede within first line of text
	nl := strings.IndexRune(t, '\n')
	if nl > -1 {
		t = t[:nl]
	}

	// Strip certain HTML tags
	t = titleElementReg.ReplaceAllString(t, "")

	// Strip URL protocols
	t = urlReg.ReplaceAllString(t, "")

	// Strip image URL, leaving only alt text
	t = imgReg.ReplaceAllString(t, " $1 ")

	// Find lede within first sentence
	punc := strings.Index(t, ". ")
	if punc > -1 {
		t = t[:punc+iAdj]
	}
	punc = stringmanip.IndexRune(t, '。')
	if punc > -1 {
		c := []rune(t)
		t = string(c[:punc+iAdj])
	}
	punc = stringmanip.IndexRune(t, '?')
	if punc > -1 {
		c := []rune(t)
		t = string(c[:punc+iAdj])
	}

	return t
}

// TruncToWord truncates the given text to the provided limit.
func TruncToWord(s string, l int) (string, bool) {
	truncated := false
	c := []rune(s)
	if len(c) > l {
		truncated = true
		s = string(c[:l])
		spaceIdx := strings.LastIndexByte(s, ' ')
		if spaceIdx > -1 {
			s = s[:spaceIdx]
		}
	}
	return s, truncated
}
