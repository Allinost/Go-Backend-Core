package developmentmanager

import "testing"

func TestWithinDir(t *testing.T) {
	base := `C:\repo`
	cases := map[string]bool{
		`C:\repo`:           true,
		`C:\repo\docs`:      true,
		`C:\repo\docs\a.md`: true,
		`C:\repository`:     false,
		`C:\repo\..\other`:  false,
	}
	for target, want := range cases {
		if got := withinDir(base, target); got != want {
			t.Errorf("withinDir(%q)=%v，期望 %v", target, got, want)
		}
	}
}

func TestResolveAssetDirRejectsUnsafe(t *testing.T) {
	p := &Project{RepoPath: "."}
	for _, uri := range []string{"", "../etc", "docs/../../etc", "/abs/path"} {
		if _, err := resolveAssetDir(p, uri); err == nil {
			t.Errorf("URI %q 应被拒绝", uri)
		}
	}
}
