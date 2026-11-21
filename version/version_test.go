package version

import (
	"strings"
	"testing"
)

func TestShort(t *testing.T) {
	if got := Short(); got != Version {
		t.Fatalf("Short() = %q, 期望 %q", got, Version)
	}
}

func TestString(t *testing.T) {
	s := String()
	for _, want := range []string{"RequestFlow", Version, shortCommit(Commit)} {
		if !strings.Contains(s, want) {
			t.Fatalf("String() = %q 未包含 %q", s, want)
		}
	}
}

func TestDetail(t *testing.T) {
	d := Detail()
	for _, want := range []string{"版本:", "提交哈希:", "构建时间:", "Go 版本:", "目标平台:", Version, Commit} {
		if !strings.Contains(d, want) {
			t.Fatalf("Detail() 未包含 %q:\n%s", want, d)
		}
	}
}

func TestShortCommit(t *testing.T) {
	cases := map[string]string{
		"abcdef1234567890": "abcdef1",
		"abc123":           "abc123",
		"":                 "",
	}
	for in, want := range cases {
		if got := shortCommit(in); got != want {
			t.Fatalf("shortCommit(%q) = %q, 期望 %q", in, got, want)
		}
	}
}
