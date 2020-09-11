package httpheaders

import (
	"net/http"
	"testing"

	"nhooyr.io/websocket/internal/test/assert"
)

func TestIsToken(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		value string
		ok    bool
	}{
		{
			name: "empty",
			ok:   false,
		},
		{
			name:  "basic",
			value: "Hello_World_123",
			ok:    true,
		},
		{
			name:  "separator",
			value: "hello world",
			ok:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, "valid", tc.ok, IsToken(tc.value))
		})
	}
}

func TestContainsToken(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		tokens Tokens
		token  string
		ok     bool
	}{
		{
			name: "empty",
			ok:   false,
		},
		{
			name:   "contains",
			tokens: Tokens{"foo", "bar", "baz"}, token: "bar",
			ok: true,
		},
		{
			name:   "case-insensitive",
			tokens: Tokens{"foo", "bar", "baz"}, token: "BAR",
			ok: true,
		},
		{
			name:   "does not contain",
			tokens: Tokens{"foo", "bar", "baz"}, token: "qux",
			ok: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := ContainsToken(tc.tokens, tc.token)
			assert.Equal(t, "contains", tc.ok, ok)
		})
	}
}

func TestVerifyIsToken(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		header http.Header
		key    string
		token  string
		err    bool
	}{
		{
			name: "empty",
			err:  true,
		},
		{
			name:   "empty value",
			header: header("Key", ""), key: "Key", token: "foo",
			err: true,
		},
		{
			name:   "extra value",
			header: header("Key", "foo", "bar"), key: "Key", token: "foo",
			err: true,
		},
		{
			name:   "invalid value",
			header: header("Key", "foo; bar"), key: "Key", token: "foo",
			err: true,
		},
		{
			name:   "does not match",
			header: header("Key", "bar"), key: "Key", token: "foo",
			err: true,
		},
		{
			name:   "matches",
			header: header("Key", "foo"), key: "Key", token: "foo",
			err: false,
		},
		{
			name:   "case-insensitive",
			header: header("Key", "Foo"), key: "KEY", token: "FOO",
			err: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assertError(t, tc.err, VerifyIsToken(tc.header, tc.key, tc.token))
		})
	}
}

func TestVerifyContainsToken(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		header http.Header
		key    string
		token  string
		err    bool
	}{
		{
			name:  "empty",
			key:   "key",
			token: "foo",
			err:   true,
		},
		{
			name:   "contains",
			header: header("Key", "foo, bar, baz"), key: "Key", token: "bar",
			err: false,
		},
		{
			name:   "case-insensitive",
			header: header("Key", "Foo, Bar, Baz"), key: "KEY", token: "BAR",
			err: false,
		},
		{
			name:   "contains with invalid headers",
			header: header("Key", "foo", "bar; baz", "qux"), key: "Key", token: "qux",
		},
		{
			name:   "does not contain",
			header: header("Key", "foo", "bar", "baz"), key: "Key", token: "qux",
			err: true,
		},
		{
			name:   "does not contain with invalid header",
			header: header("Key", "foo", "bar; baz"), key: "Key", token: "bar",
			err: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assertError(t, tc.err, VerifyContainsToken(tc.header, tc.key, tc.token))
		})
	}
}

func TestParseTokenLists(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		values []string
		tokens Tokens
		err    bool
	}{
		{
			name: "empty",
		},
		{
			name:   "one token",
			values: []string{"foo"},
			tokens: Tokens{"foo"},
		},
		{
			name:   "two tokens",
			values: []string{"foo, bar"},
			tokens: Tokens{"foo", "bar"},
		},
		{
			name:   "two values",
			values: []string{"foo", "bar"},
			tokens: Tokens{"foo", "bar"},
		},
		{
			name:   "three tokens",
			values: []string{"foo, bar, qux"},
			tokens: Tokens{"foo", "bar", "qux"},
		},
		{
			name:   "three values",
			values: []string{"foo", "bar, baz", "qux"},
			tokens: Tokens{"foo", "bar", "baz", "qux"},
		},
		{
			name:   "no spaces",
			values: []string{"foo,bar,qux"},
			tokens: Tokens{"foo", "bar", "qux"},
		},
		{
			name:   "extra spaces",
			values: []string{"  foo   ,  bar  , qux   "},
			tokens: Tokens{"foo", "bar", "qux"},
		},
		{
			name:   "extra commas",
			values: []string{" ,,,   foo   ,", ",  bar , , , qux , "},
			tokens: Tokens{"foo", "bar", "qux"},
		},
		{
			name:   "empty value",
			values: []string{""},
			err:    true,
		},
		{
			name:   "invalid separator",
			values: []string{"foo", "bar; qux"},
			err:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := ParseTokenLists(tc.values)
			assertError(t, tc.err, err)
			assert.Equal(t, "tokens", tc.tokens, tokens)
		})
	}
}

func TestReadString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input string
		value string
		rest  string
		err   bool
	}{
		{
			name: "empty",
			err:  true,
		},
		{
			name:  "token",
			input: "hello world",
			value: "hello", rest: "world",
		},
		{
			name:  "string",
			input: `"hello world" rest`,
			value: "hello world", rest: "rest",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			value, rest, err := ReadString(tc.input)
			assert.Equal(t, "value", tc.value, value)
			assert.Equal(t, "rest", tc.rest, rest)
			assertError(t, tc.err, err)
		})
	}
}

func TestReadToken(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input string
		token string
		rest  string
		err   bool
	}{
		{
			name: "empty",
			err:  true,
		},
		{
			name:  "leading space",
			input: " hello",
			err:   true,
		},
		{
			name:  "token",
			input: "hello",
			token: "hello",
		},
		{
			name:  "trailing space",
			input: "hello    ",
			token: "hello",
		},
		{
			name:  "space separator",
			input: "hello      world",
			token: "hello", rest: "world",
		},
		{
			name:  "special separator",
			input: "hello, world",
			token: "hello", rest: ", world",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token, rest, err := ReadToken(tc.input)
			assert.Equal(t, "token", tc.token, token)
			assert.Equal(t, "rest", tc.rest, rest)
			assertError(t, tc.err, err)
		})
	}
}

func TestReadQuotedString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input string
		value string
		rest  string
		err   bool
	}{
		{
			name: "empty",
			err:  true,
		},
		{
			name:  "leading space",
			input: ` "foo"`,
			err:   true,
		},
		{
			name:  "missing opening quote",
			input: `foo"`,
			err:   true,
		},
		{
			name:  "missing closing quote",
			input: `"foo`,
			err:   true,
		},
		{
			name:  "missing escaped char",
			input: `"foo\`,
			err:   true,
		},
		{
			name:  "invalid escaped char",
			input: `"foo\` + string(rune(128)) + `bar"`,
			err:   true,
		},
		{
			name:  "string",
			input: `"hello world"`,
			value: "hello world",
		},
		{
			name:  "escapes",
			input: `"\"hello world\""`,
			value: `"hello world"`,
		},
		{
			name:  "trailing space",
			input: `"hello"    `,
			value: "hello",
		},
		{
			name:  "space separator",
			input: `"hello world"      rest`,
			value: "hello world", rest: "rest",
		},
		{
			name:  "special separator",
			input: `"hello, world", rest`,
			value: "hello, world", rest: ", rest",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			value, rest, err := ReadQuotedString(tc.input)
			assert.Equal(t, "value", tc.value, value)
			assert.Equal(t, "rest", tc.rest, rest)
			assertError(t, tc.err, err)
		})
	}
}

func TestFormatString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "empty",
			input:  ``,
			output: `""`,
		},
		{
			name:   "token",
			input:  `hello-world`,
			output: `hello-world`,
		},
		{
			name:   "string",
			input:  `hello world`,
			output: `"hello world"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, "value", tc.output, FormatString(tc.input))
		})
	}
}

func TestQuoteString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "empty",
			input:  ``,
			output: `""`,
		},
		{
			name:   "simple",
			input:  `hello world`,
			output: `"hello world"`,
		},
		{
			name:   "quotes",
			input:  `"hello world"`,
			output: `"\"hello world\""`,
		},
		{
			name:   "backslash",
			input:  `hello\world`,
			output: `"hello\\world"`,
		},
		{
			name:   "null character",
			input:  `hello ` + string(rune(0)) + ` world`,
			output: `"hello \` + string(rune(0)) + ` world"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, "value", tc.output, QuoteString(tc.input))
		})
	}
}

func header(key string, values ...string) http.Header {
	h := make(http.Header)
	for _, v := range values {
		h.Add(key, v)
	}
	return h
}

func assertError(t testing.TB, expectError bool, err error) {
	t.Helper()
	if expectError {
		assert.Error(t, err)
	} else {
		assert.Success(t, err)
	}
}
