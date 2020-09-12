package wsheaders

import (
	"net/http"
	"testing"

	"nhooyr.io/websocket/internal/test/assert"
)

func TestSetProtocols(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		protos []string
		header http.Header
	}{
		{
			name:   "one",
			protos: []string{"foo"},
			header: header(ProtocolKey, "foo"),
		},
		{
			name:   "many",
			protos: []string{"foo", "bar", "baz", "qux"},
			header: header(ProtocolKey, "foo, bar, baz, qux"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := make(http.Header)
			SetProtocols(h, tc.protos...)
			assert.Equal(t, "header", tc.header, h)
		})
	}
}

func TestSelectProtocol(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		header    http.Header
		supported Protocols
		proto     string
		ok        bool
	}{
		{
			name:   "basic",
			header: header(ProtocolKey, "foo, bar"), supported: Protocols{"bar", "foo"},
			proto: "bar", ok: true,
		},
		{
			name:   "multiple values",
			header: header(ProtocolKey, "foo, bar"), supported: Protocols{"bar"},
			proto: "bar", ok: true,
		},
		{
			name:   "multiple headers",
			header: header(ProtocolKey, "foo", "bar"), supported: Protocols{"bar"},
			proto: "bar", ok: true,
		},
		{
			name:   "case-insensitive",
			header: header(ProtocolKey, "FOO, BAR"), supported: Protocols{"bar"},
			proto: "bar", ok: true,
		},
		{
			name:   "fallback",
			header: header(ProtocolKey, "foo, baz"), supported: Protocols{"bar", "baz"},
			proto: "baz", ok: true,
		},
		{
			name:  "empty",
			proto: "", ok: false,
		},
		{
			name:   "none",
			header: header(ProtocolKey, "foo, baz"), supported: Protocols{"bar", "qux"},
			proto: "", ok: false,
		},
		{
			name:   "invalid",
			header: header(ProtocolKey, "foo; baz"), supported: Protocols{"foo"},
			proto: "", ok: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			proto, ok := SelectProtocol(tc.header, tc.supported)
			assert.Equal(t, "proto", tc.proto, proto)
			assert.Equal(t, "ok", tc.ok, ok)
		})
	}
}

func TestVerifyProtocol(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		header    http.Header
		supported Protocols
		proto     string
		err       bool
	}{
		{
			name: "empty",
		},
		{
			name:   "basic",
			header: header(ProtocolKey, "foo"), supported: Protocols{"foo", "bar"},
			proto: "foo",
		},
		{
			name:   "case-insensitive",
			header: header(ProtocolKey, "BAR"), supported: Protocols{"foo", "bar"},
			proto: "bar",
		},
		{
			name:   "invalid",
			header: header(ProtocolKey, "foo; bar"), supported: Protocols{"bar"},
			err: true,
		},
		{
			name:   "mismatch",
			header: header(ProtocolKey, "foo"), supported: Protocols{"bar"},
			err: true,
		},
		{
			name:   "multiple values",
			header: header(ProtocolKey, "foo, baz"), supported: Protocols{"foo"},
			err: true,
		},
		{
			name:   "multiple headers",
			header: header(ProtocolKey, "foo", "baz"), supported: Protocols{"foo"},
			err: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			proto, err := VerifyProtocol(tc.header, tc.supported)
			assertError(t, tc.err, err)
			assert.Equal(t, "proto", tc.proto, proto)
		})
	}
}
