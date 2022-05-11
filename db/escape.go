/*
 * Copyright © 2019-2022 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package db

import (
	"strings"
)

type EscapeContext int

const (
	EscapeSimple EscapeContext = iota
)

func (_ EscapeContext) SQLEscape(d DialectType, s string) (string, error) {
	builder := strings.Builder{}
	switch d {
	case DialectSQLite:
		builder.WriteRune('\'')
		for _, c := range s {
			if c == '\'' {
				builder.WriteString("''")
			} else {
				builder.WriteRune(c)
			}
		}
		builder.WriteRune('\'')
	case DialectMySQL:
		builder.WriteRune('\'')
		for _, c := range s {
			switch c {
			case 0:
				builder.WriteString("\\0")
			case '\'':
				builder.WriteString("\\'")
			case '"':
				builder.WriteString("\\\"")
			case '\b':
				builder.WriteString("\\b")
			case '\n':
				builder.WriteString("\\n")
			case '\r':
				builder.WriteString("\\r")
			case '\t':
				builder.WriteString("\\t")
			case '\\':
				builder.WriteString("\\\\")
			default:
				builder.WriteRune(c)
			}
		}
		builder.WriteRune('\'')
	}
	return builder.String(), nil
}
