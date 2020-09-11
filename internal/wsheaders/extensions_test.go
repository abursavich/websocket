package wsheaders

import (
	"net/http"
	"testing"

	"nhooyr.io/websocket/internal/test/assert"
)

func TestSetExtensions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		exts   Extensions
		header http.Header
	}{
		{
			name: "one",
			exts: Extensions{{
				Name: "foo",
				Params: []ExtensionParam{
					{Name: "bar"},
					{Name: "qux", Value: "42"},
				},
			}},
			header: header(ExtensionsKey, "foo; bar; qux=42"),
		},
		{
			name: "many",
			exts: Extensions{
				{
					Name:   "foo",
					Params: []ExtensionParam{{Name: "bar"}},
				},
				{
					Name: "foo",
				},
				{
					Name: "bar",
					Params: []ExtensionParam{
						{Name: "foo", Value: "bar"},
						{Name: "qux", Value: "42"},
					},
				},
			},
			header: header(ExtensionsKey, "foo; bar, foo, bar; foo=bar; qux=42"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := make(http.Header)
			SetExtensions(h, tc.exts...)
			assert.Equal(t, "header", tc.header, h)
		})
	}
}

func TestParseExtensions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		headers []string
		exts    Extensions
		err     bool
	}{
		{
			name: "empty",
		},
		{
			name:    "emptyHeader",
			headers: []string{""},
			err:     true,
		},
		{
			name:    "emptyHeaderWithCommas",
			headers: []string{" , , "},
			err:     true,
		},
		{
			name:    "extension",
			headers: []string{"permessage-foo"},
			exts:    Extensions{{Name: "permessage-foo"}},
		},
		{
			name:    "extensionWithExtraCommas",
			headers: []string{",permessage-foo, "},
			exts:    Extensions{{Name: "permessage-foo"}},
		},
		{
			name:    "invalidName",
			headers: []string{"???"},
			err:     true,
		},
		{
			name:    "param",
			headers: []string{"permessage-foo; use_y"},
			exts: Extensions{{
				Name:   "permessage-foo",
				Params: []ExtensionParam{{Name: "use_y"}},
			}},
		},
		{
			name:    "invalidParam",
			headers: []string{`permessage-foo; ???`},
			err:     true,
		},
		{
			name:    "value",
			headers: []string{"permessage-foo; x=10"},
			exts: Extensions{{
				Name:   "permessage-foo",
				Params: []ExtensionParam{{Name: "x", Value: "10"}},
			}},
		},
		{
			name:    "multipleValues",
			headers: []string{"permessage-foo; x=10; y=20"},
			exts: Extensions{{
				Name: "permessage-foo",
				Params: []ExtensionParam{
					{Name: "x", Value: "10"},
					{Name: "y", Value: "20"},
				},
			}},
		},
		{
			name:    "invalidValue",
			headers: []string{`permessage-foo; x=???`},
			err:     true,
		},
		{
			name:    "quotedValue",
			headers: []string{`permessage-foo; x="10"`},
			exts: Extensions{{
				Name:   "permessage-foo",
				Params: []ExtensionParam{{Name: "x", Value: "10"}},
			}},
		},
		{
			name:    "escapedQuotedValue",
			headers: []string{`permessage-foo; x="\1\0"`},
			exts: Extensions{{
				Name:   "permessage-foo",
				Params: []ExtensionParam{{Name: "x", Value: "10"}},
			}},
		},
		{
			name:    "invalidQuotedValue",
			headers: []string{`permessage-foo; x="???"`},
			err:     true,
		},
		{
			name:    "unclosedQuotedValue",
			headers: []string{`permessage-foo; x="TAB`},
			err:     true,
		},
		{
			name:    "multipleHeaders",
			headers: []string{"foo", "bar; baz=2"},
			exts: Extensions{
				{
					Name: "foo",
				},
				{
					Name:   "bar",
					Params: []ExtensionParam{{Name: "baz", Value: "2"}},
				},
			},
		},
		{
			name:    "multipleExtensions",
			headers: []string{"foo, bar; baz=2"},
			exts: Extensions{
				{
					Name: "foo",
				},
				{
					Name:   "bar",
					Params: []ExtensionParam{{Name: "baz", Value: "2"}},
				},
			},
		},
		{
			name:    "multipleExtensionsExtraSpaces",
			headers: []string{`   foo ,    bar  ;     baz =   2 ;   qux  = "42" ,   baz `},
			exts: Extensions{
				{
					Name: "foo",
				},
				{
					Name: "bar",
					Params: []ExtensionParam{
						{Name: "baz", Value: "2"},
						{Name: "qux", Value: "42"},
					},
				},
				{
					Name: "baz",
				},
			},
		},
		{
			name:    "multipleExtensionsNoSpaces",
			headers: []string{`foo,bar;baz=2;qux="42",baz`},
			exts: Extensions{
				{
					Name: "foo",
				},
				{
					Name: "bar",
					Params: []ExtensionParam{
						{Name: "baz", Value: "2"},
						{Name: "qux", Value: "42"},
					},
				},
				{
					Name: "baz",
				},
			},
		},
		{
			name:    "invalidExtensionsList",
			headers: []string{"foo", "bar, baz=2"},
			err:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exts, err := ParseExtensions(header(ExtensionsKey, tc.headers...))
			assertError(t, tc.err, err)
			assert.Equal(t, "extensions", tc.exts, exts)
		})
	}
}
