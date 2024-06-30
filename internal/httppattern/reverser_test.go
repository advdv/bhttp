package httppattern_test

import (
	"strings"
	"testing"

	"github.com/advdv/bhttp/internal/httppattern"
)

func TestReversing(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		in   string
		vals []string
		out  string
	}{
		{"/", []string{}, "/"},
		{"/", []string{"dd/bb"}, "/dd/bb"},
		{"/a", []string{}, "/a"},
		{"/a/", []string{}, "/a/"},
		{"/path/to/something", []string{}, "/path/to/something"},
		{"/{w1}/lit/{w2}", []string{"111", "foo"}, "/111/lit/foo"},
		{"/{w1}/lit/{w2}/", []string{"234", "bar"}, "/234/lit/bar/"},
		{"example.com/", []string{}, "example.com/"},
		{"GET /", []string{}, "/"},
		{"POST example.com/foo/345", []string{}, "example.com/foo/345"},
		{"/{$}", []string{}, "/"},
		{"DELETE example.com/a/{foo12}/{$}", []string{"777"}, "example.com/a/777/"},
		{"/foo/{$}", []string{}, "/foo/"},
		{"/{a}/foo/{rest...}", []string{"786", "d/a/f"}, "/786/foo/d/a/f"},
		{"//", []string{}, "//"},
		{"/foo///./../bar", []string{}, "/foo///./../bar"},
		{"a.com/foo//", []string{}, "a.com/foo//"},
		{"/%61%62/%7b/%", []string{}, "/ab/{/%"},
	} {
		pat, err := httppattern.ParsePattern(test.in)
		if err != nil {
			t.Fatalf("got: %v", err)
		}

		out, err := httppattern.Build(pat, test.vals...)
		if err != nil {
			t.Fatalf("got: %v", err)
		}

		if out != test.out {
			t.Errorf("got: %q, want: %q", out, test.out)
		}
	}
}

func TestReversingError(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		in       string
		vals     []string
		contains string
	}{
		{"/{w1}/lit/{w2}", []string{"111"}, `not enough values for pattern "/{w1}/lit/{w2}", expect at least: 2`},
		{"/", []string{"dd/bb", "err"}, `too many values for pattern "/", got: 2, used: 1`},
		{"/a", []string{"dd/bb"}, `too many values for pattern "/a", got: 1, used: 0`},
	} {
		pat, err := httppattern.ParsePattern(test.in)
		if err != nil {
			t.Fatalf("got: %v", err)
		}

		_, err = httppattern.Build(pat, test.vals...)
		if err == nil || !strings.Contains(err.Error(), test.contains) {
			t.Errorf("%q:\ngot %v, want error containing %q", test.in, err, test.contains)
		}
	}
}
