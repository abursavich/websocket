// Package extensions parses websocket extensions headers according to RFC 6455.
package extensions

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"nhooyr.io/websocket/internal/headers"
)

// Header is the canonical websocket extensions header key.
const Header = "Sec-WebSocket-Extensions"

// https://tools.ietf.org/html/rfc6455#section-9.1
//
// Sec-WebSocket-Extensions = extension-list
// extension-list           = 1#extension
// extension                = extension-token *( ";" extension-param )
// extension-token          = registered-token
// registered-token         = token
// extension-param          = token [ "=" (token | quoted-string) ]
//     ;When using the quoted-string syntax variant, the value
// 	   ;after quoted-string unescaping MUST conform to the
// 	   ;'token' ABNF.

// Extensions represents list of extensions.
type Extensions []Extension

func (e Extensions) String() string {
	switch n := len(e); n {
	case 0:
		return ""
	case 1:
		return e[0].String()
	default:
		elems := make([]string, n)
		for i, v := range e {
			elems[i] = v.String()
		}
		return strings.Join(elems, ", ")
	}
}

// Extension represents a named extension with an optional list of parameters.
type Extension struct {
	Name   string
	Params Params
}

func (e Extension) String() string {
	if len(e.Params) == 0 {
		return e.Name
	}
	return e.Name + "; " + e.Params.String()
}

// Params represents list of extension parameters.
type Params []Param

func (p Params) String() string {
	switch n := len(p); n {
	case 0:
		return ""
	case 1:
		return p[0].String()
	default:
		elems := make([]string, n)
		for i, v := range p {
			elems[i] = v.String()
		}
		return strings.Join(elems, "; ")
	}
}

// Param represents a named extension parameter with an optional value.
type Param struct {
	Name  string
	Value string
}

func (p Param) String() string {
	if p.Value == "" {
		return p.Name
	}
	return p.Name + "=" + p.Value
}

// ParseHeader parses the extension values in the given header.
func ParseHeader(h http.Header) (Extensions, error) {
	return Parse(h.Values(Header)...)
}

// Parse parses the given values as extension headers.
func Parse(values ...string) (Extensions, error) {
	if len(values) == 0 {
		return nil, nil
	}
	exts, err := parse(values[0])
	if err != nil {
		return nil, err
	}
	for _, val := range values[1:] {
		v, err := parse(val)
		if err != nil {
			return nil, err
		}
		exts = append(exts, v...)
	}
	return exts, nil
}

func parse(val string) (Extensions, error) {
	// extension-list           = 1#extension
	var (
		exts Extensions
		ext  Extension
		err  error
	)
	rest := trimLeftCommaOrSpace(val)
	for {
		ext, rest, err = parseExtension(rest)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q: %v", val, err)
		}
		exts = append(exts, ext)
		if rest == "" {
			return exts, nil
		}
		if rest != "" && !strings.HasPrefix(rest, ",") {
			return nil, fmt.Errorf("failed to parse %q: expecting ',': found %q", val, rest[0])
		}
		if rest = trimLeftCommaOrSpace(rest); rest == "" {
			return exts, nil
		}
	}
}

func parseExtension(s string) (ext Extension, rest string, err error) {
	// extension                = extension-token *( ";" extension-param )
	// extension-token          = registered-token
	// registered-token         = token
	ext.Name, rest, err = headers.ReadToken(s)
	if err != nil {
		return Extension{}, "", fmt.Errorf("extension name: %v", err)
	}
	for strings.HasPrefix(rest, ";") {
		rest = trimLeftSpace(rest[1:])
		var param Param
		param, rest, err = parseParam(rest)
		if err != nil {
			return Extension{}, "", err
		}
		ext.Params = append(ext.Params, param)
	}
	return ext, rest, nil
}

func parseParam(s string) (param Param, rest string, err error) {
	// extension-param          = token [ "=" (token | quoted-string) ]
	//     ;When using the quoted-string syntax variant, the value
	// 	   ;after quoted-string unescaping MUST conform to the
	// 	   ;'token' ABNF.
	param.Name, rest, err = headers.ReadToken(s)
	if err != nil {
		return Param{}, "", fmt.Errorf("parameter name: %v", err)
	}
	if !strings.HasPrefix(rest, "=") {
		return param, rest, nil
	}
	rest = trimLeftSpace(rest[1:])
	param.Value, rest, err = headers.ReadString(rest)
	if err != nil {
		return Param{}, "", fmt.Errorf("parameter value: %v", err)
	}
	if !headers.IsValidToken(param.Value) {
		return Param{}, "", fmt.Errorf("parameter value: invalid token: %q", param.Value)
	}
	return param, rest, nil
}

func trimLeftCommaOrSpace(s string) string {
	return strings.TrimLeftFunc(s, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}

func trimLeftSpace(s string) string {
	return strings.TrimLeftFunc(s, unicode.IsSpace)
}
