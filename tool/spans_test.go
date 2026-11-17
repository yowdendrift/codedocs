package main

import "testing"

func TestBraceEnd(t *testing.T) {
	cases := []struct {
		name string
		src  []string
		ext  string
		want int
	}{
		{"plain function", []string{"function a() {", "  b();", "}", "x"}, "ts", 3},
		{"object return type", []string{"export function start(m: Model): { dispose(): void } {", "  go();", "  return { dispose() {} };", "}", "x"}, "ts", 4},
		{"one-line body", []string{"func a() { b() }", "x"}, "go", 1},
	}
	for _, c := range cases {
		if got := braceEnd(c.src, 1, c.ext); got != c.want {
			t.Errorf("%s: ends at %d, want %d", c.name, got, c.want)
		}
	}
	long := []string{"func a() {"}
	for i := 0; i < 500; i++ {
		long = append(long, "\tb()")
	}
	long = append(long, "}")
	if got := braceEnd(long, 1, "go"); got != len(long) {
		t.Errorf("a 502-line function ends at %d, want %d", got, len(long))
	}
}
