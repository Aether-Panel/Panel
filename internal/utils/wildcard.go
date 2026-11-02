package utils

import (
	"regexp"
	"strings"
)

func CompareWildcard(source, match string) bool {
	if match == "" || match == "*" {
		return true
	}
	if strings.Contains(match, "*") {
		regex := WildCardToRegexp(match)
		return matchRegex(regex, source)
	}

	return source == match
}

// WildCardToRegexp converts a wildcard pattern to a regular expression pattern.
func WildCardToRegexp(pattern string) string {
	var result strings.Builder
	for i, literal := range strings.Split(pattern, "*") {

		// Replace * with .*
		if i > 0 {
			result.WriteString(".*")
		}

		// Quote any regular expression meta characters in the
		// literal text.
		result.WriteString(regexp.QuoteMeta(literal))
	}
	return result.String()
}

// matchRegex matches value against a regex made by WildCardToRegexp. It must not convert
// it again: that quoted the ".*" too, so no pattern with a * in it ever matched.
func matchRegex(regex, value string) bool {
	result, _ := regexp.MatchString("^"+regex+"$", value)
	return result
}
