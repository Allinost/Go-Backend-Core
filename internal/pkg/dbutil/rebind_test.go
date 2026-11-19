package dbutil

import "testing"

func TestRebind(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`SELECT * FROM t WHERE a = ? AND b = ?`, `SELECT * FROM t WHERE a = $1 AND b = $2`},
		{`SELECT '?' AS q, "?" AS d, ` + "`?`" + `, a FROM t WHERE b = ?`, `SELECT '?' AS q, "?" AS d, ` + "`?`" + `, a FROM t WHERE b = $1`},
		{"SELECT 1 -- ?\nWHERE a = ?", "SELECT 1 -- ?\nWHERE a = $1"},
		{"SELECT 1 /* ? */ WHERE a = ?", "SELECT 1 /* ? */ WHERE a = $1"},
		{`SELECT 'it''s ?' WHERE a = ?`, `SELECT 'it''s ?' WHERE a = $1`},
		{`INSERT INTO t (a) VALUES (?)`, `INSERT INTO t (a) VALUES ($1)`},
	}
	for _, c := range cases {
		if got := Rebind(c.in); got != c.want {
			t.Errorf("Rebind(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
