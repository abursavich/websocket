package wsheaders

import (
	"net/http"
	"testing"

	"nhooyr.io/websocket/internal/test/assert"
)

func TestSetConnection(t *testing.T) {
	h := make(http.Header)
	SetConnection(h)
	assert.Equal(t, "value", header("Connection", "Upgrade"), h)
}

func TestSetUpgrade(t *testing.T) {
	h := make(http.Header)
	SetUpgrade(h)
	assert.Equal(t, "value", header("Upgrade", "WebSocket"), h)
}

func TestSetVersion(t *testing.T) {
	h := make(http.Header)
	SetVersion(h, 13)
	assert.Equal(t, "value", header(VersionKey, "13"), h)
}

func TestSetChallenge(t *testing.T) {
	h := make(http.Header)
	SetChallenge(h, []byte("hello"))
	assert.Equal(t, "set", header(ChallengeKey, "aGVsbG8="), h)
}

func TestVerifyConnection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		header http.Header
		err    bool
	}{
		{
			name:   "simple",
			header: header("Connection", "Upgrade"),
		},
		{
			name:   "case-insensitive",
			header: header("Connection", "UpGrAdE"),
		},
		{
			name:   "multiple tokens",
			header: header("Connection", "Keep-Alive, Upgrade"),
		},
		{
			name:   "multiple values",
			header: header("Connection", "Keep-Alive", "Upgrade"),
		},
		{
			name: "missing",
			err:  true,
		},
		{
			name:   "invalid",
			header: header("Connection", "Keep-Alive; Upgrade"),
			err:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assertError(t, tc.err, VerifyConnection(tc.header))
		})
	}
}

func TestVerifyClientUpgrade(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		header http.Header
		err    bool
	}{
		{
			name:   "simple",
			header: header("Upgrade", "WebSocket"),
		},
		{
			name:   "case-insensitive",
			header: header("Upgrade", "WeBsOcKeT"),
		},
		{
			name:   "first token",
			header: header("Upgrade", "WebSocket, FooBar"),
		},
		{
			name:   "second token",
			header: header("Upgrade", "FooBar, WebSocket"),
		},
		{
			name:   "first value",
			header: header("Upgrade", "WebSocket", "FooBar"),
		},
		{
			name:   "second value",
			header: header("Upgrade", "FooBar", "WebSocket"),
		},
		{
			name: "missing",
			err:  true,
		},
		{
			name:   "invalid",
			header: header("Upgrade", "FooBar WebSocket"),
			err:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assertError(t, tc.err, VerifyClientUpgrade(tc.header))
		})
	}
}

func TestVerifyServerUpgrade(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		header http.Header
		err    bool
	}{
		{
			name:   "simple",
			header: header("Upgrade", "WebSocket"),
		},
		{
			name:   "case-insensitive",
			header: header("Upgrade", "WeBsOcKeT"),
		},
		{
			name: "missing",
			err:  true,
		},
		{
			name:   "invalid",
			header: header("Upgrade", "WebSocket FooBar"),
			err:    true,
		},
		{
			name:   "multiple tokens",
			header: header("Upgrade", "WebSocket, FooBar"),
			err:    true,
		},
		{
			name:   "multiple values",
			header: header("Upgrade", "WebSocket", "FooBar"),
			err:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assertError(t, tc.err, VerifyServerUpgrade(tc.header))
		})
	}
}

func TestGetVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		header  http.Header
		version byte
		err     bool
	}{
		{
			name:    "simple",
			header:  header(VersionKey, "13"),
			version: 13,
		},
		{
			name: "missing",
			err:  true,
		},
		{
			name:   "invalid",
			header: header(VersionKey, "13; 42"),
			err:    true,
		},
		{
			name:   "multiple tokens",
			header: header(VersionKey, "13, 42"),
			err:    true,
		},
		{
			name:   "multiple values",
			header: header(VersionKey, "13", "42"),
			err:    true,
		},
		{
			name:   "underflow",
			header: header(VersionKey, "-1"),
			err:    true,
		},
		{
			name:   "overflow",
			header: header(VersionKey, "256"),
			err:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := GetVersion(tc.header)
			assertError(t, tc.err, err)
			assert.Equal(t, "version", tc.version, v)
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
