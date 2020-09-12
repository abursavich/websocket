package wsheaders

import (
	"encoding/base64"
	"net/http"
	"testing"

	"nhooyr.io/websocket/internal/test/assert"
)

// Source: https://tools.ietf.org/html/rfc6455#section-1.3
const (
	validChallenge = "dGhlIHNhbXBsZSBub25jZQ=="
	validAccept    = "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
)

var validChallengeBuf, _ = base64.StdEncoding.DecodeString(validChallenge)

func TestGetChallenge(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		header http.Header
		value  []byte
		err    bool
	}{
		{
			name:   "success",
			header: header(ChallengeKey, validChallenge),
			value:  validChallengeBuf,
		},
		{
			name: "missing",
			err:  true,
		},
		{
			name:   "invalid",
			header: header(ChallengeKey, "ABCDEF"),
			err:    true,
		},
		{
			name:   "multiple",
			header: header(ChallengeKey, validChallenge, validChallenge),
			err:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := GetChallenge(tc.header)
			assertError(t, tc.err, err)
			assert.Equal(t, "version", tc.value, v)
		})
	}
}

func TestSetAccept(t *testing.T) {
	t.Parallel()

	got := make(http.Header)
	SetAccept(got, validChallengeBuf)

	exp := header(AcceptKey, validAccept)

	assert.Equal(t, "header", exp, got)
}

func TestVerifyAccept(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		header http.Header
		value  string
		err    bool
	}{
		{
			name:   "success",
			header: header(AcceptKey, validAccept),
			value:  validAccept,
		},
		{
			name: "missing",
			err:  true,
		},
		{
			name:   "empty",
			header: header(AcceptKey, ""),
			err:    true,
		},
		{
			name:   "invalid",
			header: header(AcceptKey, "ABCDEF"),
			err:    true,
		},
		{
			name:   "multiple",
			header: header(AcceptKey, validAccept, validAccept),
			err:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assertError(t, tc.err, VerifyAccept(tc.header, validChallengeBuf))
		})
	}
}
