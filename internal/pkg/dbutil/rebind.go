// Package dbutil 提供跨数据库 SQL 小工具。
package dbutil

import (
	"strconv"
	"strings"
)

// Rebind 将 MySQL 风格的 ? 占位符转换为 PostgreSQL 的 $1..$n。
// 会跳过单引号、双引号、反引号、行注释和块注释中的 ?。
func Rebind(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 8)

	arg := 0
	inSingle := false
	inDouble := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(query); i++ {
		c := query[i]

		if inLineComment {
			b.WriteByte(c)
			if c == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			b.WriteByte(c)
			if c == '*' && i+1 < len(query) && query[i+1] == '/' {
				b.WriteByte('/')
				i++
				inBlockComment = false
			}
			continue
		}
		if inSingle {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(query) {
				b.WriteByte(query[i+1])
				i++
				continue
			}
			if c == '\'' {
				if i+1 < len(query) && query[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		if inDouble {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(query) {
				b.WriteByte(query[i+1])
				i++
				continue
			}
			if c == '"' {
				if i+1 < len(query) && query[i+1] == '"' {
					b.WriteByte('"')
					i++
					continue
				}
				inDouble = false
			}
			continue
		}
		if inBacktick {
			b.WriteByte(c)
			if c == '`' {
				inBacktick = false
			}
			continue
		}

		switch {
		case c == '-' && i+1 < len(query) && query[i+1] == '-':
			b.WriteString("--")
			i++
			inLineComment = true
		case c == '/' && i+1 < len(query) && query[i+1] == '*':
			b.WriteString("/*")
			i++
			inBlockComment = true
		case c == '\'':
			b.WriteByte(c)
			inSingle = true
		case c == '"':
			b.WriteByte(c)
			inDouble = true
		case c == '`':
			b.WriteByte(c)
			inBacktick = true
		case c == '?':
			arg++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(arg))
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
