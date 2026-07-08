package pagination

import (
	"testing"
)

func FuzzParseOffset(f *testing.F) {
	f.Add(0, 0)
	f.Add(1, 20)
	f.Add(-1, -1)
	f.Add(100, 200)
	f.Fuzz(func(t *testing.T, page, size int) {
		offset, limit := ParseOffset(page, size)
		if offset < 0 {
			t.Errorf("ParseOffset(%d,%d) offset=%d < 0", page, size, offset)
		}
		if limit < 1 || limit > 100 {
			t.Errorf("ParseOffset(%d,%d) limit=%d out of [1,100]", page, size, limit)
		}
	})
}

func FuzzParsePage(f *testing.F) {
	f.Add(0, 0)
	f.Add(1, 20)
	f.Add(-5, -10)
	f.Fuzz(func(t *testing.T, page, size int) {
		p, s := ParsePage(page, size)
		if p < 1 {
			t.Errorf("ParsePage(%d,%d) page=%d < 1", page, size, p)
		}
		if s < 1 || s > 100 {
			t.Errorf("ParsePage(%d,%d) size=%d out of [1,100]", page, size, s)
		}
	})
}

func FuzzParseCursor(f *testing.F) {
	f.Add("", 0)
	f.Add("abc", 20)
	f.Add("cursor123", -1)
	f.Fuzz(func(t *testing.T, cursor string, limit int) {
		l := ParseCursor(cursor, limit)
		if l < 1 || l > 100 {
			t.Errorf("ParseCursor(%q,%d) limit=%d out of [1,100]", cursor, limit, l)
		}
	})
}
