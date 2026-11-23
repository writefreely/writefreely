package db

import (
    "fmt"
)

// PGQueryWrap converts a prepared statement from MySQL/SQLite format, which
// uses ? placeholders, into PostgreSQL format, which uses $1, $2 ... $n.
func PGQueryWrap(q string) string {
    output := ""
    escaped := false
    var escapeChar byte = 0
    ctr := 0

    for i := range len(q) {
        if escaped {
            if q[i] == escapeChar {
                escaped = false
                escapeChar = 0
            }
        } else if q[i] == '\'' || q[i] == '`' || q[i] == '"' {
            escaped = true
            escapeChar = q[i]
        }

        if q[i] == '?' && !escaped {
            ctr += 1
            output += fmt.Sprintf("$%d", ctr)
        } else {
            output += string(q[i])
        }
    }

    return output
}
