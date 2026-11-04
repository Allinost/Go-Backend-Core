package goodser

import "testing"

func TestNormalizeImageURL(t *testing.T) {
	ptr := func(value string) *string { return &value }

	cases := []struct {
		name string
		in   *string
		want string
	}{
		{"nil", nil, ""},
		{"empty", ptr(""), ""},
		{"external untouched", ptr("https://picsum.photos/200/200?random=1"), "https://picsum.photos/200/200?random=1"},
		{
			"signed storage url stripped",
			ptr("https://storage.example.com/go-backend-core/goodser/images/a.jpg?X-Amz-Signature=abc"),
			"https://storage.example.com/go-backend-core/goodser/images/a.jpg",
		},
		{
			"plain storage url kept",
			ptr("https://storage.example.com/go-backend-core/goodser/images/a.jpg"),
			"https://storage.example.com/go-backend-core/goodser/images/a.jpg",
		},
	}

	for _, tc := range cases {
		got := normalizeImageURL(tc.in)
		if tc.in == nil {
			if got != nil {
				t.Errorf("%s: 期望 nil，实际 %v", tc.name, *got)
			}
			continue
		}
		if got == nil || *got != tc.want {
			t.Errorf("%s: 期望 %q，实际 %v", tc.name, tc.want, got)
		}
	}
}
