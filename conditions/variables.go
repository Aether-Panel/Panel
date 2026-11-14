package conditions

import (
	"fmt"
	"regexp"
	"strings"
)

// variableToken is a ${name} reference in a condition
var variableToken = regexp.MustCompile(`\$\{([^{}]+)\}`)

// celIdentifier is a name CEL can use as a variable as it is
var celIdentifier = regexp.MustCompile(`^[_a-zA-Z][_a-zA-Z0-9]*$`)

// SubstituteVariables replaces ${name} in a condition with the variable name from data (or
// the os/arch constants), so a template can write in_path("java${javaversion}") the way it
// writes ${} everywhere else. The value never becomes CEL code:
//   - inside a string literal it is escaped for that literal ("java${v}" becomes "java17");
//     a raw literal (r"...") cannot hold escapes, so it is closed, joined with the value as
//     an escaped string and opened again;
//   - outside a literal ${name} becomes the variable itself (CEL already knows it), or the
//     value as a quoted string when name is not a valid CEL name.
//
// Unknown names and bytes literals (b"...") are left as they are.
func SubstituteVariables(condition string, data map[string]interface{}) string {
	if !strings.Contains(condition, "${") {
		return condition
	}

	var out strings.Builder
	i := 0
	for i < len(condition) {
		next := strings.IndexAny(condition[i:], `"'`)
		if next < 0 {
			out.WriteString(replaceOutsideLiteral(condition[i:], data))
			break
		}
		start := i + next
		out.WriteString(replaceOutsideLiteral(condition[i:start], data))

		raw, isBytes := literalPrefix(condition[:start])
		quote := condition[start : start+1]
		if strings.HasPrefix(condition[start:], strings.Repeat(quote, 3)) {
			quote = strings.Repeat(quote, 3)
		}
		bodyStart := start + len(quote)
		end := literalEnd(condition, bodyStart, quote, raw)

		out.WriteString(quote)
		body := condition[bodyStart:end]
		if isBytes {
			out.WriteString(body)
		} else {
			out.WriteString(replaceInsideLiteral(body, data, quote, raw))
		}
		if end < len(condition) {
			out.WriteString(quote)
			end += len(quote)
		}
		i = end
	}
	return out.String()
}

// lookupVariable finds name in data, then in the global constants (os, arch)
func lookupVariable(name string, data map[string]interface{}) (interface{}, bool) {
	if value, ok := data[name]; ok {
		return value, true
	}
	value, ok := GlobalConstantValues[name]
	return value, ok
}

func replaceOutsideLiteral(part string, data map[string]interface{}) string {
	return variableToken.ReplaceAllStringFunc(part, func(token string) string {
		name := token[2 : len(token)-1]
		value, ok := lookupVariable(name, data)
		if !ok {
			return token
		}
		if celIdentifier.MatchString(name) {
			return name
		}
		return `"` + escapeCELString(fmt.Sprint(value)) + `"`
	})
}

func replaceInsideLiteral(body string, data map[string]interface{}, quote string, raw bool) string {
	return variableToken.ReplaceAllStringFunc(body, func(token string) string {
		value, ok := lookupVariable(token[2:len(token)-1], data)
		if !ok {
			return token
		}
		escaped := escapeCELString(fmt.Sprint(value))
		if raw {
			return quote + ` + "` + escaped + `" + r` + quote
		}
		return escaped
	})
}

// literalPrefix reads the letters right before a quote: r/R makes the literal raw, b/B
// makes it bytes (and "br" both)
func literalPrefix(before string) (raw, isBytes bool) {
	j := len(before)
	for j > 0 && isIdentifierChar(before[j-1]) {
		j--
	}
	switch strings.ToLower(before[j:]) {
	case "r":
		return true, false
	case "b":
		return false, true
	case "br":
		return true, true
	}
	return false, false
}

func isIdentifierChar(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// literalEnd is where the literal whose text starts at start closes (the index of its
// closing quote), or the end of the condition when it never does
func literalEnd(condition string, start int, quote string, raw bool) int {
	for j := start; j < len(condition); j++ {
		if !raw && condition[j] == '\\' {
			j++
			continue
		}
		if strings.HasPrefix(condition[j:], quote) {
			return j
		}
	}
	return len(condition)
}

// escapeCELString escapes text for any CEL string literal that is not raw
func escapeCELString(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				_, _ = fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
