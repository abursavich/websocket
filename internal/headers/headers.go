// Package headers provides utilities for parsing and formatting
// header values according to RFC 2616.
package headers

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// https://tools.ietf.org/html/rfc2616#section-2.1
//
// *rule
//    The character "*" preceding an element indicates repetition. The
//    full form is "<n>*<m>element" indicating at least <n> and at most
//    <m> occurrences of element. Default values are 0 and infinity so
//    that "*(element)" allows any number, including zero; "1*element"
//    requires at least one; and "1*2element" allows one or two.
//
// #rule
//    A construct "#" is defined, similar to "*", for defining lists of
//    elements. The full form is "<n>#<m>element" indicating at least
//    <n> and at most <m> elements, each separated by one or more commas
//    (",") and OPTIONAL linear white space (LWS). This makes the usual
//    form of lists very easy; a rule such as
//       ( *LWS element *( *LWS "," *LWS element ))
//    can be shown as
//       1#element
//    Wherever this construct is used, null elements are allowed, but do
//    not contribute to the count of elements present. That is,
//    "(element), , (element) " is permitted, but counts as only two
//    elements. Therefore, where at least one element is required, at
//    least one non-null element MUST be present. Default values are 0
//    and infinity so that "#element" allows any number, including zero;
//    "1#element" requires at least one; and "1#2element" allows one or
//    two.

// https://tools.ietf.org/html/rfc2616#section-2.2
//
// OCTET          = <any 8-bit sequence of data>
// CHAR           = <any US-ASCII character (octets 0 - 127)>
// UPALPHA        = <any US-ASCII uppercase letter "A".."Z">
// LOALPHA        = <any US-ASCII lowercase letter "a".."z">
// ALPHA          = UPALPHA | LOALPHA
// DIGIT          = <any US-ASCII digit "0".."9">
// CTL            = <any US-ASCII control character
// 				    (octets 0 - 31) and DEL (127)>
// CR             = <US-ASCII CR, carriage return (13)>
// LF             = <US-ASCII LF, linefeed (10)>
// SP             = <US-ASCII SP, space (32)>
// HT             = <US-ASCII HT, horizontal-tab (9)>
// <">            = <US-ASCII double-quote mark (34)>
//
// CRLF           = CR LF
//
// LWS            = [CRLF] 1*( SP | HT )
//
// TEXT           = <any OCTET except CTLs,
//                  but including LWS>
//
// HEX            = "A" | "B" | "C" | "D" | "E" | "F"
//                | "a" | "b" | "c" | "d" | "e" | "f" | DIGIT
//
// token          = 1*<any CHAR except CTLs or separators>
// separators     = "(" | ")" | "<" | ">" | "@"
// 			      | "," | ";" | ":" | "\" | <">
// 			      | "/" | "[" | "]" | "?" | "="
// 			      | "{" | "}" | SP | HT
//
// comment        = "(" *( ctext | quoted-pair | comment ) ")"
// ctext          = <any TEXT excluding "(" and ")">
//
// quoted-string  = ( <"> *(qdtext | quoted-pair ) <"> )
// qdtext         = <any TEXT except <">>
//
// quoted-pair    = "\" CHAR

// Tokens represents a list of tokens.
type Tokens []string

func (s Tokens) String() string {
	return strings.Join(s, ", ")
}

// Contains returns a bool indicating if the given case-insensitive
// token exists in the list of tokens.
func (s Tokens) Contains(token string) bool {
	for _, v := range s {
		if strings.EqualFold(v, token) {
			return true
		}
	}
	return false
}

// IsValidToken returns a bool indicating if the given token is valid.
func IsValidToken(token string) bool {
	if token == "" {
		return false
	}
	for i, n := 0, len(token); i < n; i++ {
		if !isToken(token[i]) {
			return false
		}
	}
	return true
}

// ParseTokenList parses the value as a non-empty list of comma-separated tokens.
func ParseTokenList(value string) (Tokens, error) {
	var (
		tokens Tokens
		token  string
		err    error
	)
	rest := trimLeftCommaOrSpace(value)
	for {
		token, rest, err = ReadToken(rest)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q: %v", value, err)
		}
		tokens = append(tokens, token)
		if rest == "" {
			return tokens, nil
		}
		if rest != "" && !strings.HasPrefix(rest, ",") {
			return nil, fmt.Errorf("failed to parse %q: expecting ',': found %q", value, rest[0])
		}
		if rest = trimLeftCommaOrSpace(rest); rest == "" {
			return tokens, nil
		}
	}
}

// ParseTokenLists parses the values as non-empty lists of comma-separated tokens.
func ParseTokenLists(values []string) (Tokens, error) {
	if len(values) == 0 {
		return nil, nil
	}
	tokens, err := ParseTokenList(values[0])
	if err != nil {
		return nil, err
	}
	for _, val := range values[1:] {
		v, err := ParseTokenList(val)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, v...)
	}
	return tokens, nil
}

// ReadToken reads a token from the given string. It returns the token and the rest
// of the given string with leading whitespace removed or any error encountered.
func ReadToken(s string) (token, rest string, err error) {
	// token          = 1*<any CHAR except CTLs or separators>
	if s == "" {
		return "", "", errors.New("expecting token")
	}
	for i, n := 0, len(s); i < n; i++ {
		if !isToken(s[i]) {
			if i == 0 {
				return "", "", fmt.Errorf("expecting token: found %q", s[i])
			}
			return s[:i], trimLeftSpace(s[i:]), nil
		}
	}
	return s, "", nil
}

// ReadString reads a token or quoted string from the given string. It returns the value
// and the rest of the given string with leading whitespace removed or any error encountered.
func ReadString(s string) (value, rest string, err error) {
	if s == "" {
		return "", "", errors.New("expecting token or quoted string")
	}
	if s[0] == '"' {
		return ReadQuotedString(s)
	}
	return ReadToken(s)
}

// ReadQuotedString reads a quoted string from the given string. It returns the unquoted string
// and the rest of the given string with leading whitespace removed or any error encountered.
func ReadQuotedString(s string) (str, rest string, err error) {
	if s == "" {
		return "", "", errors.New("expecting quoted string")
	}
	if s[0] != '"' {
		return "", "", fmt.Errorf("expecting opening quote: found %q", s[0])
	}
	escapes := 0
	for i, n := 1, len(s); i < n; i++ {
		switch s[i] {
		case '\\':
			escapes++
			i++
		case '"':
			return unescape(s[1:i], escapes), trimLeftSpace(s[i+1:]), nil
		}
	}
	return "", "", errors.New("expecting closing quote")
}

func unescape(s string, escapes int) string {
	if escapes == 0 {
		return s
	}
	buf := make([]byte, 0, len(s)-escapes)
	for i, n := 0, len(s); i < n; i++ {
		if s[i] == '\\' {
			i++
		}
		buf = append(buf, s[i])
	}
	return string(buf)
}

// FormatString returns the formatted parameter specified by the given attribute
// and value or any error encountered. It is an error if attribute is not a valid token.
func FormatString(s string) string {
	if !IsValidToken(s) {
		return QuoteString(s)
	}
	return s
}

// QuoteString returns the quoted version of the given string.
func QuoteString(s string) string {
	buf := make([]byte, 2*len(s)+2)
	buf[0] = '"'
	k := 1
	for i, n := 0, len(s); i < n; i++ {
		b := s[i]
		if !isToken(b) {
			buf[k] = '\\'
			k++
		}
		buf[k] = b
		k++
	}
	buf[k] = '"'
	return string(buf[:k+1])
}

// ReadParameter reads a parameter from the given string. It returns the parameter's
// attribute, the parameter's value, and the rest of the given string with leading
// whitespace removed or any error encountered.
func ReadParameter(s string) (attribute, value, rest string, err error) {
	// https://tools.ietf.org/html/rfc2616#section-3.6
	//
	// Parameters are in  the form of attribute/value pairs.
	//     parameter               = attribute "=" value
	//     attribute               = token
	//     value                   = token | quoted-string
	attribute, rest, err = ReadToken(s)
	if err != nil {
		return "", "", "", err
	}
	if !strings.HasPrefix(rest, "=") {
		if rest == "" {
			return "", "", "", errors.New("expecting '='")
		}
		return "", "", "", fmt.Errorf("expecting '=': found %q", rest[0])
	}
	rest = trimLeftSpace(rest[1:])
	value, rest, err = ReadString(rest)
	if err != nil {
		return "", "", "", err
	}
	return attribute, value, rest, nil
}

// FormatParameter returns the formatted parameter specified by the given attribute
// and value or any error encountered. It is an error if attribute is not a valid token.
func FormatParameter(attribute, value string) (string, error) {
	if !IsValidToken(attribute) {
		return "", fmt.Errorf("parameter attribute: invalid token: %q", attribute)
	}
	return attribute + "=" + FormatString(value), nil
}

func trimLeftCommaOrSpace(s string) string {
	return strings.TrimLeftFunc(s, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}

func trimLeftSpace(s string) string {
	return strings.TrimLeftFunc(s, unicode.IsSpace)
}

func isToken(b byte) bool {
	// CHAR           = <any US-ASCII character (octets 0 - 127)>
	// token          = 1*<any CHAR except CTLs or separators>
	return b <= 127 && !isControl(b) && !isSeparator(b)
}

func isControl(b byte) bool {
	// CTL            = <any US-ASCII control character
	//                  (octets 0 - 31) and DEL (127)>
	return b <= 31 || b == 127
}

func isSeparator(b byte) bool {
	// separators     = "(" | ")" | "<" | ">" | "@"
	// 			      | "," | ";" | ":" | "\" | <">
	// 			      | "/" | "[" | "]" | "?" | "="
	// 			      | "{" | "}" | SP | HT
	switch b {
	case '(', ')', '<', '>', '@',
		',', ';', ':', '\\', '"',
		'/', '[', ']', '?', '=',
		'{', '}', ' ', '\t':
		return true
	default:
		return false
	}
}
