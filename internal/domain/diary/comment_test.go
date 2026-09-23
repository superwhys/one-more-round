package diary

import (
	"strings"
	"testing"
)

func TestCommentValidate(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"plain", "今晚这局真好看", true},
		{"trimmed", "  留一句  ", true},
		{"empty", "", false},
		{"blank", "   ", false},
		{"limit", strings.Repeat("局", 500), true},
		{"too long", strings.Repeat("局", 500) + "🧡", false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := &Comment{Body: tt.body}
			if (c.Validate() == nil) != tt.ok {
				t.Fatalf("body %q: %v", tt.body, c.Validate())
			}
		})
	}
}

func TestCommentRoot(t *testing.T) {
	empty := ""
	parent := "parent"
	if !(Comment{}).Root() || !(Comment{ParentID: &empty}).Root() {
		t.Fatal("empty parent should be a root comment")
	}
	if (Comment{ParentID: &parent}).Root() {
		t.Fatal("reply treated as root")
	}
}
