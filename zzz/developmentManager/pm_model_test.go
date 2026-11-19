package developmentmanager

import "testing"

func TestCanTransitionTask(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{TaskStatusTodo, TaskStatusDoing, true},
		{TaskStatusTodo, TaskStatusDone, false},
		{TaskStatusDoing, TaskStatusReview, true},
		{TaskStatusDoing, TaskStatusDone, true},
		{TaskStatusReview, TaskStatusDone, true},
		{TaskStatusDone, TaskStatusDoing, true},
		{TaskStatusCancelled, TaskStatusTodo, true},
		{TaskStatusCancelled, TaskStatusDone, false},
		{"unknown", TaskStatusTodo, false},
	}
	for _, c := range cases {
		if got := CanTransitionTask(c.from, c.to); got != c.want {
			t.Errorf("CanTransitionTask(%s,%s)=%v want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	if v, err := parseDate(""); err != nil || v != nil {
		t.Fatalf("empty date: v=%v err=%v", v, err)
	}
	if v, err := parseDate("2026-11-19"); err != nil || v == nil || v.Format("2006-01-02") != "2026-11-19" {
		t.Fatalf("valid date: v=%v err=%v", v, err)
	}
	if _, err := parseDate("2026/11/19"); err == nil {
		t.Fatal("invalid date should fail")
	}
}

func TestParseDateTime(t *testing.T) {
	if v, err := parseDateTime("2026-11-19T10:00:00Z"); err != nil || v == nil {
		t.Fatalf("rfc3339: v=%v err=%v", v, err)
	}
	if v, err := parseDateTime("2026-11-19 10:00:00"); err != nil || v == nil {
		t.Fatalf("datetime: v=%v err=%v", v, err)
	}
	if _, err := parseDateTime("not-a-time"); err == nil {
		t.Fatal("invalid datetime should fail")
	}
}

func TestNormalizeID(t *testing.T) {
	zero := int64(0)
	positive := int64(3)
	if normalizeID(nil) != nil || normalizeID(&zero) != nil {
		t.Fatal("nil/zero should normalize to nil")
	}
	if got := normalizeID(&positive); got == nil || *got != 3 {
		t.Fatalf("positive id: %v", got)
	}
}
