// +build !js

package websocket

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"nhooyr.io/websocket/internal/errd"
	"nhooyr.io/websocket/internal/wsheaders"
)

// AcceptOptions represents Accept's options.
type AcceptOptions struct {
	// Subprotocols lists the WebSocket subprotocols that Accept will negotiate with the client.
	// The empty subprotocol will always be negotiated as per RFC 6455. If you would like to
	// reject it, close the connection when c.Subprotocol() == "".
	Subprotocols []string

	// InsecureSkipVerify is used to disable Accept's origin verification behaviour.
	//
	// You probably want to use OriginPatterns instead.
	InsecureSkipVerify bool

	// OriginPatterns lists the host patterns for authorized origins.
	// The request host is always authorized.
	// Use this to enable cross origin WebSockets.
	//
	// i.e javascript running on example.com wants to access a WebSocket server at chat.example.com.
	// In such a case, example.com is the origin and chat.example.com is the request host.
	// One would set this field to []string{"example.com"} to authorize example.com to connect.
	//
	// Each pattern is matched case insensitively against the request origin host
	// with filepath.Match.
	// See https://golang.org/pkg/path/filepath/#Match
	//
	// Please ensure you understand the ramifications of enabling this.
	// If used incorrectly your WebSocket server will be open to CSRF attacks.
	//
	// Do not use * as a pattern to allow any origin, prefer to use InsecureSkipVerify instead
	// to bring attention to the danger of such a setting.
	OriginPatterns []string

	// CompressionMode controls the compression mode.
	// Defaults to CompressionDisabled.
	//
	// See docs on CompressionMode for details.
	CompressionMode CompressionMode

	// CompressionThreshold controls the minimum size of a message before compression is applied.
	//
	// Defaults to 512 bytes for CompressionNoContextTakeover and 128 bytes
	// for CompressionContextTakeover.
	CompressionThreshold int
}

// Accept accepts a WebSocket handshake from a client and upgrades the
// the connection to a WebSocket.
//
// Accept will not allow cross origin requests by default.
// See the InsecureSkipVerify and OriginPatterns options to allow cross origin requests.
//
// Accept will write a response to w on all errors.
func Accept(w http.ResponseWriter, r *http.Request, opts *AcceptOptions) (*Conn, error) {
	return accept(w, r, opts)
}

func accept(w http.ResponseWriter, r *http.Request, opts *AcceptOptions) (_ *Conn, err error) {
	defer errd.Wrap(&err, "failed to accept WebSocket connection")

	if opts == nil {
		opts = &AcceptOptions{}
	}
	opts = &*opts

	challenge, errCode, err := verifyClientRequest(w, r)
	if err != nil {
		http.Error(w, err.Error(), errCode)
		return nil, err
	}

	if !opts.InsecureSkipVerify {
		err = authenticateOrigin(r, opts.OriginPatterns)
		if err != nil {
			if errors.Is(err, filepath.ErrBadPattern) {
				log.Printf("websocket: %v", err)
				err = errors.New(http.StatusText(http.StatusForbidden))
			}
			http.Error(w, err.Error(), http.StatusForbidden)
			return nil, err
		}
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		err = errors.New("http.ResponseWriter does not implement http.Hijacker")
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
		return nil, err
	}

	wsheaders.SetUpgrade(w.Header())
	wsheaders.SetConnection(w.Header())
	wsheaders.SetAccept(w.Header(), challenge)

	subproto, ok := wsheaders.SelectProtocol(r.Header, opts.Subprotocols)
	if ok {
		wsheaders.SetProtocols(w.Header(), subproto)
	}

	exts, _ := wsheaders.ParseExtensions(r.Header)
	copts, ok := selectDeflate(opts.CompressionMode, exts)
	if ok {
		wsheaders.SetExtensions(w.Header(), copts.extension())
	}

	w.WriteHeader(http.StatusSwitchingProtocols)
	// See https://github.com/nhooyr/websocket/issues/166
	if ginWriter, ok := w.(interface{ WriteHeaderNow() }); ok {
		ginWriter.WriteHeaderNow()
	}

	netConn, brw, err := hj.Hijack()
	if err != nil {
		err = fmt.Errorf("failed to hijack connection: %w", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil, err
	}

	// https://github.com/golang/go/issues/32314
	b, _ := brw.Reader.Peek(brw.Reader.Buffered())
	brw.Reader.Reset(io.MultiReader(bytes.NewReader(b), netConn))

	return newConn(connConfig{
		subprotocol:    subproto,
		rwc:            netConn,
		client:         false,
		copts:          copts,
		flateThreshold: opts.CompressionThreshold,

		br: brw.Reader,
		bw: brw.Writer,
	}), nil
}

func verifyClientRequest(w http.ResponseWriter, r *http.Request) (challenge []byte, errCode int, err error) {
	if !r.ProtoAtLeast(1, 1) {
		return nil, http.StatusUpgradeRequired, fmt.Errorf("WebSocket protocol violation: handshake request must be at least HTTP/1.1: %q", r.Proto)
	}

	if err := wsheaders.VerifyConnection(r.Header); err != nil {
		wsheaders.SetConnection(w.Header())
		wsheaders.SetUpgrade(w.Header())
		return nil, http.StatusUpgradeRequired, fmt.Errorf("WebSocket protocol violation: %v", err)
	}

	if err := wsheaders.VerifyClientUpgrade(r.Header); err != nil {
		wsheaders.SetConnection(w.Header())
		wsheaders.SetUpgrade(w.Header())
		return nil, http.StatusUpgradeRequired, fmt.Errorf("WebSocket protocol violation: %v", err)
	}

	if r.Method != "GET" {
		return nil, http.StatusMethodNotAllowed, fmt.Errorf("WebSocket protocol violation: handshake request method is not GET but %q", r.Method)
	}

	if v, err := wsheaders.GetVersion(r.Header); v != 13 {
		wsheaders.SetVersion(w.Header(), 13)
		if err != nil {
			return nil, http.StatusUpgradeRequired, fmt.Errorf("WebSocket protocol violation: %v", err)
		}
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported WebSocket protocol version (only 13 is supported): %d", v)
	}

	if challenge, err = wsheaders.GetChallenge(r.Header); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("WebSocket protocol violation: %v", err)
	}
	return challenge, 0, nil
}

func authenticateOrigin(r *http.Request, originHosts []string) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}

	u, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("failed to parse Origin header %q: %w", origin, err)
	}

	if strings.EqualFold(r.Host, u.Host) {
		return nil
	}

	for _, hostPattern := range originHosts {
		matched, err := match(hostPattern, u.Host)
		if err != nil {
			return fmt.Errorf("failed to parse filepath pattern %q: %w", hostPattern, err)
		}
		if matched {
			return nil
		}
	}
	return fmt.Errorf("request Origin %q is not authorized for Host %q", origin, r.Host)
}

func match(pattern, s string) (bool, error) {
	return filepath.Match(strings.ToLower(pattern), strings.ToLower(s))
}

func selectDeflate(mode CompressionMode, exts wsheaders.Extensions) (*compressionOptions, bool) {
	if mode == CompressionDisabled {
		return nil, false
	}
	for _, ext := range exts {
		switch ext.Name {
		case "permessage-deflate":
			if copts, ok := acceptDeflate(mode, ext.Params); ok {
				return copts, true
			}
		}
	}
	return nil, false
}

func acceptDeflate(mode CompressionMode, params []wsheaders.ExtensionParam) (*compressionOptions, bool) {
	copts := mode.opts()
	seen := make(map[string]bool)
	for _, p := range params {
		if seen[p.Name] {
			return nil, false
		}
		seen[p.Name] = true

		switch p.Name {
		case "client_no_context_takeover":
			if p.Value == "" {
				copts.clientNoContextTakeover = true
				continue
			}
		case "server_no_context_takeover":
			if p.Value == "" {
				copts.serverNoContextTakeover = true
				continue
			}
		case "client_max_window_bits":
			if p.Value == "" || isValidWindowBits(p.Value) {
				// We can't adjust the deflate window, but decoding with a larger window is acceptable.
				continue
			}
		case "server_max_window_bits":
			if p.Value == "15" {
				continue
			}
		}
		return nil, false
	}
	return copts, true
}
