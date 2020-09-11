package wsheaders

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"nhooyr.io/websocket/internal/httpheaders"
)

// ExtensionsKey is the canonical websocket extensions header key.
const ExtensionsKey = "Sec-WebSocket-Extensions"

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

// SetExtensions sets the extensions header to the extensions.
func SetExtensions(header http.Header, extensions ...Extension) {
	header.Set(ExtensionsKey, Extensions(extensions).String())
}

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
	Params []ExtensionParam
}

func (e Extension) String() string {
	if len(e.Params) == 0 {
		return e.Name
	}
	s := make([]string, len(e.Params)+1)
	s[0] = e.Name
	for i, v := range e.Params {
		s[i+1] = v.String()
	}
	return strings.Join(s, "; ")
}

// ExtensionParam represents a named extension parameter with an optional value.
type ExtensionParam struct {
	Name  string
	Value string
}

func (p ExtensionParam) String() string {
	if p.Value == "" {
		return p.Name
	}
	return p.Name + "=" + p.Value
}

// ParseExtensions parses the extension values in the given header.
func ParseExtensions(header http.Header) (Extensions, error) {
	values := header.Values(ExtensionsKey)
	if len(values) == 0 {
		return nil, nil
	}
	exts, err := parseExtensionList(values[0])
	if err != nil {
		return nil, fmt.Errorf("invalid "+ExtensionsKey+" header: %v", err)
	}
	for _, val := range values[1:] {
		v, err := parseExtensionList(val)
		if err != nil {
			return nil, fmt.Errorf("invalid "+ExtensionsKey+" header: %v", err)
		}
		exts = append(exts, v...)
	}
	return exts, nil
}

func parseExtensionList(val string) (Extensions, error) {
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
	ext.Name, rest, err = httpheaders.ReadToken(s)
	if err != nil {
		return Extension{}, "", fmt.Errorf("extension name: %v", err)
	}
	for strings.HasPrefix(rest, ";") {
		rest = trimLeftSpace(rest[1:])
		var param ExtensionParam
		param, rest, err = parseParam(rest)
		if err != nil {
			return Extension{}, "", err
		}
		ext.Params = append(ext.Params, param)
	}
	return ext, rest, nil
}

func parseParam(s string) (param ExtensionParam, rest string, err error) {
	// extension-param          = token [ "=" (token | quoted-string) ]
	//     ;When using the quoted-string syntax variant, the value
	// 	   ;after quoted-string unescaping MUST conform to the
	// 	   ;'token' ABNF.
	param.Name, rest, err = httpheaders.ReadToken(s)
	if err != nil {
		return ExtensionParam{}, "", fmt.Errorf("parameter name: %v", err)
	}
	if !strings.HasPrefix(rest, "=") {
		return param, rest, nil
	}
	rest = trimLeftSpace(rest[1:])
	param.Value, rest, err = httpheaders.ReadString(rest)
	if err != nil {
		return ExtensionParam{}, "", fmt.Errorf("parameter value: %v", err)
	}
	if !httpheaders.IsToken(param.Value) {
		return ExtensionParam{}, "", fmt.Errorf("parameter value: invalid token: %q", param.Value)
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
