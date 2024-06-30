package bhttp

import (
	"fmt"

	"github.com/advdv/bhttp/internal/httppattern"
	"github.com/samber/lo"
)

// Reverser keeps track of named patterns and  allows building URLS.
type Reverser struct {
	pats map[string]*httppattern.Pattern
}

// NewReverser inits the reverser.
func NewReverser() *Reverser {
	return &Reverser{make(map[string]*httppattern.Pattern)}
}

// Reverse reverses the named pattern into a url.
func (r Reverser) Reverse(name string, vals ...string) (string, error) {
	pat, ok := r.pats[name]
	if !ok {
		return "", fmt.Errorf("no pattern named: %q, got: %v", name, lo.Keys(r.pats)) //nolint:goerr113
	}

	res, err := httppattern.Build(pat, vals...)
	if err != nil {
		return "", fmt.Errorf("failed to build: %w", err)
	}

	return res, nil
}

// Named is a convenience method that panics if naming the pattern fails.
func (r Reverser) Named(name, s string) string {
	s, err := r.NamedPattern(name, s)
	if err != nil {
		panic("clserve: " + err.Error())
	}

	return s
}

// NamedPattern will parse 's' as a path pattern while returning it as well.
func (r Reverser) NamedPattern(name, s string) (string, error) {
	if _, exists := r.pats[name]; exists {
		return s, fmt.Errorf("pattern with name %q already exists", name) //nolint:goerr113
	}

	pat, err := httppattern.ParsePattern(s)
	if err != nil {
		return s, fmt.Errorf("failed to parse pattern: %w", err)
	}

	r.pats[name] = pat

	return s, nil
}
