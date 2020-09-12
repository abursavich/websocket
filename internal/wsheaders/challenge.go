// Package wsheaders parses websocket headers according to RFC 6455.
package wsheaders

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	// ChallengeKey is the canonical websocket challenge header key.
	ChallengeKey = "Sec-WebSocket-Key"

	// AcceptKey is the canonical websocket accept header key.
	AcceptKey = "Sec-WebSocket-Accept"
)

var salt = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func hash(challenge []byte) string {
	h := sha1.New()
	enc := base64.NewEncoder(base64.StdEncoding, h)
	enc.Write(challenge)
	enc.Close()
	h.Write(salt)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// GetChallenge returns the Sec-WebSocket-Key header value. It's an error if the
// header doesn't exist, has multiple values, or isn't base64.
func GetChallenge(h http.Header) ([]byte, error) {
	// Sec-WebSocket-Key = base64-value-non-empty
	// base64-value-non-empty = (1*base64-data [ base64-padding ]) | base64-padding
	// base64-data      = 4base64-character
	// base64-padding   = (2base64-character "==") | (3base64-character "=")
	// base64-character = ALPHA | DIGIT | "+" | "/"
	v, err := getOne(h, ChallengeKey)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(v)))
	if err != nil {
		return nil, fmt.Errorf("invalid "+ChallengeKey+" header: value %q is not base64", v)
	}
	return b, nil
}

// SetChallenge sets the Sec-WebSocket-Key header to the given value.
func SetChallenge(h http.Header, value []byte) {
	h.Set(ChallengeKey, base64.StdEncoding.EncodeToString(value))
}

// SetAccept sets the Sec-WebSocket-Accept response header to
// a hash of the given challenge.
func SetAccept(h http.Header, challenge []byte) {
	h.Set(AcceptKey, hash(challenge))
}

// VerifyAccept returns an error if the Sec-WebSocket-Accept response header
// is not a hash of the given challenge. It is an error if the header appears
// more than once.
func VerifyAccept(h http.Header, challenge []byte) error {
	v, err := getOne(h, AcceptKey)
	if err != nil {
		return err
	}
	if v != hash(challenge) {
		return fmt.Errorf("invalid "+AcceptKey+" header: hash %q does not match key %q", v, challenge)
	}
	return nil
}
