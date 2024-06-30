package httppattern

import (
	"fmt"
	"strings"
)

// Pattern exposes the private type /net/http.pattern.
type Pattern pattern

// ParsePattern parses 's' as a patterned route for Go 1.22's ServeMux.
func ParsePattern(s string) (*Pattern, error) {
	p, err := parsePattern(s)

	return (*Pattern)(p), err
}

// Build constructs a full url given the pattern 'pat' and 'vals' for wildcards.
func Build(pat *Pattern, vals ...string) (string, error) {
	var res strings.Builder

	// always write the host (if any)
	res.WriteString(pat.host)

	vidx, vused := 0, 0

	for _, seg := range pat.segments {
		res.WriteString("/")

		// Paths ending in "{$}" are represented with the literal segment "/".
		// For example, the path "a/{$}" is represented as a literal segment "a" followed
		// by a literal segment "/".
		if seg.s == "/" && !seg.wild {
			break
		}

		if seg.wild {
			if seg.multi && seg.s == "" {
				// Paths ending in '/' are represented with an anonymous "..." wildcard.
				// For example, the path "a/" is represented as a literal segment "a" followed
				// by a segment with multi==true.
				if vidx <= (len(vals) - 1) {
					// if there is another value we add it since a trailingn '/' acts as a
					// wildcard.
					res.WriteString(vals[vidx])

					vused++
				}

				break // otherwise, we're done
			}

			// if there are not enough values we error.
			if vidx > (len(vals) - 1) {
				//nolint:goerr113
				return "", fmt.Errorf("not enough values for pattern %q, expect at least: %d", pat.str, vidx+1)
			}

			res.WriteString(vals[vidx])

			vidx++
			vused++

			continue
		}

		res.WriteString(seg.s)
	}

	if len(vals) != vused {
		//nolint:goerr113
		return res.String(), fmt.Errorf("too many values for pattern %q, got: %d, used: %d", pat.str, len(vals), vused)
	}

	return res.String(), nil
}
