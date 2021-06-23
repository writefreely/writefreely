/*
 * Copyright © 2021 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import "testing"

func TestApplyBasicMarkdown(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		result string
	}{
		{"empty", "", ""},
		{"plain", "Hello, World!", "Hello, World!"},
		{"multibyte", "こんにちは", `こんにちは`},
		{"bold", "**안녕하세요**", `<strong>안녕하세요</strong>`},
		{"link", "[WriteFreely](https://writefreely.org)", `<a href="https://writefreely.org" rel="nofollow">WriteFreely</a>`},
		{"date", "12. April", `12. April`},
		{"table", "| Hi | There |", `| Hi | There |`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := applyBasicMarkdown([]byte(test.in))
			if res != test.result {
				t.Errorf("%s: wanted %s, got %s", test.name, test.result, res)
			}
		})
	}
}
