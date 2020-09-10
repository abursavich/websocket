// +build !js

package websocket

import (
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket/internal/test/assert"
)

func TestBadDials(t *testing.T) {
	t.Parallel()

	t.Run("badReq", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name string
			url  string
			opts *DialOptions
			rand readerFunc
		}{
			{
				name: "badURL",
				url:  "://noscheme",
			},
			{
				name: "badURLScheme",
				url:  "ftp://nhooyr.io",
			},
			{
				name: "badHTTPClient",
				url:  "ws://nhooyr.io",
				opts: &DialOptions{
					HTTPClient: &http.Client{
						Timeout: time.Minute,
					},
				},
			},
			{
				name: "badTLS",
				url:  "wss://totallyfake.nhooyr.io",
			},
			{
				name: "badReader",
				rand: func(p []byte) (int, error) {
					return 0, io.EOF
				},
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				defer cancel()

				if tc.rand == nil {
					tc.rand = rand.Reader.Read
				}

				_, _, err := dial(ctx, tc.url, tc.opts, tc.rand)
				assert.Error(t, err)
			})
		}
	})

	t.Run("badResponse", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		_, _, err := Dial(ctx, "ws://example.com", &DialOptions{
			HTTPClient: mockHTTPClient(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					Body: ioutil.NopCloser(strings.NewReader("hi")),
				}, nil
			}),
		})
		assert.Contains(t, err, "failed to WebSocket dial: expected handshake response status code 101 but got 0")
	})

	t.Run("badBody", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		rt := func(r *http.Request) (*http.Response, error) {
			h := http.Header{}
			h.Set("Connection", "Upgrade")
			h.Set("Upgrade", "websocket")
			h.Set("Sec-WebSocket-Accept", secWebSocketAccept(r.Header.Get("Sec-WebSocket-Key")))

			return &http.Response{
				StatusCode: http.StatusSwitchingProtocols,
				Header:     h,
				Body:       ioutil.NopCloser(strings.NewReader("hi")),
			}, nil
		}

		_, _, err := Dial(ctx, "ws://example.com", &DialOptions{
			HTTPClient: mockHTTPClient(rt),
		})
		assert.Contains(t, err, "response body is not a io.ReadWriteCloser")
	})
}

func Test_verifyServerResponse(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		dopts    *DialOptions
		response func(w http.ResponseWriter)
		copts    *compressionOptions
		success  bool
	}{
		{
			name: "badStatus",
			response: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusOK)
			},
			success: false,
		},
		{
			name: "badConnection",
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "???")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "badUpgrade",
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "???")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "badSecWebSocketAccept",
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Accept", "xd")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "badSecWebSocketProtocol",
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Protocol", "xd")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "unsupportedExtension",
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "meow")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "unsupportedDeflateParam",
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate; meow")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "duplicateDeflateParam",
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate; client_no_context_takeover; client_no_context_takeover")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "extraDeflateExtension",
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Add("Sec-WebSocket-Extensions", "permessage-deflate; client_no_context_takeover")
				w.Header().Add("Sec-WebSocket-Extensions", "permessage-deflate")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "success",
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: true,
		},
		{
			name: "deflate",
			dopts: &DialOptions{
				CompressionMode: CompressionContextTakeover,
			},
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			copts: &compressionOptions{
				clientNoContextTakeover: false,
				serverNoContextTakeover: false,
			},
			success: true,
		},
		{
			name: "deflateClientNoContextTakeover",
			dopts: &DialOptions{
				CompressionMode: CompressionContextTakeover,
			},
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate; client_no_context_takeover")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			copts: &compressionOptions{
				clientNoContextTakeover: true,
				serverNoContextTakeover: false,
			},
			success: true,
		},
		{
			name: "deflateClientNoContextTakeoverInvalid",
			dopts: &DialOptions{
				CompressionMode: CompressionContextTakeover,
			},
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate; client_no_context_takeover=invalid")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "deflateServerNoContextTakeover",
			dopts: &DialOptions{
				CompressionMode: CompressionContextTakeover,
			},
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate; server_no_context_takeover")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			copts: &compressionOptions{
				clientNoContextTakeover: false,
				serverNoContextTakeover: true,
			},
			success: true,
		},
		{
			name: "deflateServerNoContextTakeoverInvalid",
			dopts: &DialOptions{
				CompressionMode: CompressionContextTakeover,
			},
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate; server_no_context_takeover=invalid")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "deflateServerMaxWindowBits7",
			dopts: &DialOptions{
				CompressionMode: CompressionContextTakeover,
			},
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate; server_max_window_bits=7")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "deflateServerMaxWindowBits8",
			dopts: &DialOptions{
				CompressionMode: CompressionContextTakeover,
			},
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate; server_max_window_bits=8")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			copts: &compressionOptions{
				clientNoContextTakeover: false,
				serverNoContextTakeover: false,
			},
			success: true,
		},
		{
			name: "deflateServerMaxWindowBits15",
			dopts: &DialOptions{
				CompressionMode: CompressionContextTakeover,
			},
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate; server_max_window_bits=15")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			copts: &compressionOptions{
				clientNoContextTakeover: false,
				serverNoContextTakeover: false,
			},
			success: true,
		},
		{
			name: "deflateServerMaxWindowBits16",
			dopts: &DialOptions{
				CompressionMode: CompressionContextTakeover,
			},
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate; server_max_window_bits=16")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "deflateServerMaxWindowBitsInvalid",
			dopts: &DialOptions{
				CompressionMode: CompressionContextTakeover,
			},
			response: func(w http.ResponseWriter) {
				w.Header().Set("Connection", "Upgrade")
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate; server_max_window_bits=invalid")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			key, err := secWebSocketKey(rand.Reader)
			assert.Success(t, err)
			req.Header.Set("Sec-WebSocket-Key", key)

			w := httptest.NewRecorder()
			tc.response(w)
			resp := w.Result()
			if resp.Header.Get("Sec-WebSocket-Accept") == "" {
				resp.Header.Set("Sec-WebSocket-Accept", secWebSocketAccept(key))
			}

			var opts DialOptions
			if tc.dopts != nil {
				opts = *tc.dopts
			}
			copts, err := verifyServerResponse(&opts, key, resp)
			if tc.success {
				assert.Success(t, err)
			} else {
				assert.Error(t, err)
			}
			assert.Equal(t, "compression options", tc.copts, copts)
		})
	}
}

func mockHTTPClient(fn roundTripperFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
