// Package httpheaders provides utilities for parsing and formatting
// header values according to RFC 2616.
package httpheaders

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
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
//                  (octets 0 - 31) and DEL (127)>
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
//                | "," | ";" | ":" | "\" | <">
//                | "/" | "[" | "]" | "?" | "="
//                | "{" | "}" | SP | HT
//
// comment        = "(" *( ctext | quoted-pair | comment ) ")"
// ctext          = <any TEXT excluding "(" and ")">
//
// quoted-string  = ( <"> *(qdtext | quoted-pair ) <"> )
// qdtext         = <any TEXT except <">>
//
// quoted-pair    = "\" CHAR

// Tokens is a list of tokens.
type Tokens []string

func (t Tokens) String() string {
	return strings.Join(t, ", ")
}

// IsToken returns a bool indicating if the given string is a valid token.
func IsToken(s string) bool {
	if s == "" {
		return false
	}
	for i, n := 0, len(s); i < n; i++ {
		if !isToken(s[i]) {
			return false
		}
	}
	return true
}

// ContainsToken returns a bool indicating if the case-insensitive
// token exists in the list of tokens.
func ContainsToken(tokens Tokens, token string) bool {
	for _, v := range tokens {
		if strings.EqualFold(v, token) {
			return true
		}
	}
	return false
}

// VerifyIsToken returns an error if the header with the given key does not
// match the case-insensitive token. It is an error if the header value does
// not exist or cannot parsed as a non-empty comma-separated token list.
func VerifyIsToken(header http.Header, key, token string) error {
	key = http.CanonicalHeaderKey(key)
	values, ok := header[key]
	if !ok {
		return fmt.Errorf("missing %s header: must be %q", key, token)
	}
	tokens, err := ParseTokenLists(values)
	if err != nil {
		return fmt.Errorf("invalid %s header: %v", key, err)
	}
	if len(tokens) != 1 || !strings.EqualFold(tokens[0], token) {
		return fmt.Errorf("invalid %s header: %q is not %q", key, tokens, token)
	}
	return nil
}

// VerifyContainsToken returns an error if the header with the given key does not
// contain the case-insensitive token. It is an error if the header value does
// not exist or cannot parsed as a non-empty comma-separated token list.
func VerifyContainsToken(header http.Header, key, token string) error {
	key = http.CanonicalHeaderKey(key)
	values, ok := header[key]
	if !ok {
		return fmt.Errorf("missing %s header: must contain %q", key, token)
	}
	tokens, firstErr := ParseTokenList(values[0])
	if ContainsToken(tokens, token) {
		return nil
	}
	for _, value := range values[1:] {
		more, err := ParseTokenList(value)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if ContainsToken(more, token) {
			return nil
		}
		tokens = append(tokens, more...)
	}
	if firstErr != nil {
		return fmt.Errorf("invalid %s header: %v", key, firstErr)
	}
	return fmt.Errorf("invalid %s header: %q does not contain %q", key, tokens, token)
}

// ParseTokenList parses the value as a non-empty list of comma-separated tokens.
func ParseTokenList(value string) (Tokens, error) {
	var (
		tokens Tokens
		token  string
		rest   = trimLeftCommaOrSpace(value)
		err    error
	)
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

// ReadString reads a token or quoted string from the given string.
// It returns the value and the rest of the given string with leading
// whitespace removed or any error encountered.
func ReadString(s string) (value, rest string, err error) {
	if s == "" {
		return "", "", errors.New("expecting token or quoted string")
	}
	if s[0] == '"' {
		return ReadQuotedString(s)
	}
	return ReadToken(s)
}

// FormatString returns the formatted version of the given string.
// If it is a valid token, it is returned unchanged.
// Otherwise, it is returned quoted.
func FormatString(s string) string {
	if !IsToken(s) {
		return QuoteString(s)
	}
	return s
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

// ReadQuotedString reads a quoted string from the given string.
// It returns the unquoted string and the rest of the given string
// with leading whitespace removed or any error encountered.
func ReadQuotedString(s string) (str, rest string, err error) {
	// quoted-string  = ( <"> *(qdtext | quoted-pair ) <"> )
	// qdtext         = <any TEXT except <">>
	// quoted-pair    = "\" CHAR
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
			if i++; i == n {
				return "", "", errors.New("expecting escaped char")
			}
			if !isChar(s[i]) {
				return "", "", fmt.Errorf("expecting escaped char: found %q", s[i])
			}
			escapes++
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

// QuoteString returns the quoted version of the given string.
func QuoteString(s string) string {
	// quoted-string  = ( <"> *(qdtext | quoted-pair ) <"> )
	// qdtext         = <any TEXT except <">>
	// quoted-pair    = "\" CHAR
	buf := make([]byte, 2*len(s)+2)
	buf[0] = '"'
	k := 1
	for i, n := 0, len(s); i < n; i++ {
		b := s[i]
		if !isText(b) || b == '"' || b == '\\' {
			// NB: re: quoted-pair; non-TEXT is a subset of CHAR
			buf[k] = '\\'
			k++
		}
		buf[k] = b
		k++
	}
	buf[k] = '"'
	return string(buf[:k+1])
}

func trimLeftCommaOrSpace(s string) string {
	return strings.TrimLeftFunc(s, func(r rune) bool {
		b := byte(r)
		return rune(b) == r && (b == ',' || isSpace(b))
	})
}

func trimLeftSpace(s string) string {
	return strings.TrimLeftFunc(s, func(r rune) bool {
		b := byte(r)
		return rune(b) == r && isSpace(b)
	})
}

func isSpace(b byte) bool {
	// LWS            = [CRLF] 1*( SP | HT )
	switch b {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func isToken(b byte) bool {
	// token          = 1*<any CHAR except CTLs or separators>
	return isChar(b) && !isControl(b) && !isSeparator(b)
}

func isChar(b byte) bool {
	// CHAR           = <any US-ASCII character (octets 0 - 127)>
	return b <= 127
}

func isText(b byte) bool {
	// TEXT           = <any OCTET except CTLs,
	//                  but including LWS>
	return !isControl(b) || isSpace(b)
}

func isControl(b byte) bool {
	// CTL            = <any US-ASCII control character
	//                  (octets 0 - 31) and DEL (127)>
	return b <= 31 || b == 127
}

func isSeparator(b byte) bool {
	// separators     = "(" | ")" | "<" | ">" | "@"
	//                | "," | ";" | ":" | "\" | <">
	//                | "/" | "[" | "]" | "?" | "="
	//                | "{" | "}" | SP | HT
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
