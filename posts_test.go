/*
 * Copyright © 2020-2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"testing"

	"github.com/guregu/null/zero"
	"github.com/stretchr/testify/assert"
)

func TestPostSummary(t *testing.T) {
	testCases := map[string]struct {
		given    Post
		expected string
	}{
		"no special chars":          {givenPost("Content."), "Content."},
		"HTML content":              {givenPost("Content <p>with a</p> paragraph."), "Content with a paragraph."},
		"content with escaped char": {givenPost("Content&#39;s all OK."), "Content's all OK."},
		"multiline content": {givenPost(`Content
in
multiple
lines.`), "Content in multiple lines."},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := test.given.Summary()
			assert.Equal(t, test.expected, actual)
		})
	}
}

func givenPost(content string) Post {
	return Post{Title: zero.StringFrom("Title"), Content: content}
}

func TestExtractImageAltText(t *testing.T) {
	testCases := map[string]struct {
		content  string
		expected map[string]string
	}{
		"basic image": {
			"![a cat](https://example.com/cat.png)",
			map[string]string{"https://example.com/cat.png": "a cat"},
		},
		"image with title": {
			`![a cat](https://example.com/cat.png "Kitty")`,
			map[string]string{"https://example.com/cat.png": "a cat"},
		},
		"empty alt text is omitted": {
			"![](https://example.com/cat.png)",
			map[string]string{},
		},
		"whitespace-only alt text is omitted": {
			"![   ](https://example.com/cat.png)",
			map[string]string{},
		},
		"alt text is trimmed": {
			"![  a cat  ](https://example.com/cat.png)",
			map[string]string{"https://example.com/cat.png": "a cat"},
		},
		"multiple images": {
			"![cat](https://example.com/cat.png) and ![dog](https://example.com/dog.jpg)",
			map[string]string{
				"https://example.com/cat.png": "cat",
				"https://example.com/dog.jpg": "dog",
			},
		},
		"bare url has no alt text": {
			"See https://example.com/cat.png",
			map[string]string{},
		},
		"non-image markdown link ignored": {
			"[a link](https://example.com/page)",
			map[string]string{},
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, extractImageAltText(test.content))
		})
	}
}
