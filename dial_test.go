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
	"nhooyr.io/websocket/internal/wsheaders"
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
			challenge, err := wsheaders.GetChallenge(r.Header)
			if err != nil {
				return nil, err
			}

			h := http.Header{}
			wsheaders.SetConnection(h)
			wsheaders.SetUpgrade(h)
			wsheaders.SetAccept(h, challenge)

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

func Test_verifyServerHandshake(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		response func(w http.ResponseWriter)
		dopts    *DialOptions
		subproto string
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
				wsheaders.SetConnection(w.Header())
				w.Header().Set("Upgrade", "???")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "badSecWebSocketAccept",
			response: func(w http.ResponseWriter) {
				wsheaders.SetConnection(w.Header())
				wsheaders.SetUpgrade(w.Header())
				w.Header().Set(wsheaders.AcceptKey, "xd")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "badSecWebSocketProtocol",
			response: func(w http.ResponseWriter) {
				wsheaders.SetConnection(w.Header())
				wsheaders.SetUpgrade(w.Header())
				w.Header().Set(wsheaders.ProtocolKey, "xd")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "unsupportedExtension",
			response: func(w http.ResponseWriter) {
				wsheaders.SetConnection(w.Header())
				wsheaders.SetUpgrade(w.Header())
				w.Header().Set("Sec-WebSocket-Extensions", "meow")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "unsupportedDeflateParam",
			response: func(w http.ResponseWriter) {
				wsheaders.SetConnection(w.Header())
				wsheaders.SetUpgrade(w.Header())
				w.Header().Set("Sec-WebSocket-Extensions", "permessage-deflate; meow")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: false,
		},
		{
			name: "success",
			response: func(w http.ResponseWriter) {
				wsheaders.SetConnection(w.Header())
				wsheaders.SetUpgrade(w.Header())
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success: true,
		},
		{
			name: "subproto",
			dopts: &DialOptions{
				Subprotocols: []string{"BAR", "FOO"},
			},
			response: func(w http.ResponseWriter) {
				wsheaders.SetConnection(w.Header())
				wsheaders.SetUpgrade(w.Header())
				wsheaders.SetProtocols(w.Header(), "foo")
				w.WriteHeader(http.StatusSwitchingProtocols)
			},
			success:  true,
			subproto: "FOO",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/", nil)
			challenge, err := generateChallenge(rand.Reader)
			assert.Success(t, err)
			wsheaders.SetChallenge(req.Header, challenge)

			w := httptest.NewRecorder()
			tc.response(w)
			resp := w.Result()
			if resp.Header.Get(wsheaders.AcceptKey) == "" {
				wsheaders.SetAccept(resp.Header, challenge)
			}

			var opts DialOptions
			if tc.dopts != nil {
				opts = *tc.dopts
			}
			subproto, _, err := verifyServerResponse(&opts, opts.CompressionMode.opts(), challenge, resp)
			if tc.success {
				assert.Success(t, err)
			} else {
				assert.Error(t, err)
			}
			assert.Equal(t, "subprotocol", tc.subproto, subproto)
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
