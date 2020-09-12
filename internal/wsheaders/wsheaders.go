// Package wsheaders parses websocket headers according to RFC 6455.
package wsheaders

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"nhooyr.io/websocket/internal/httpheaders"
)

// VersionKey is the canonical websocket accept version key.
const VersionKey = "Sec-WebSocket-Version"

// SetConnection sets the Connection header to "Upgrade".
func SetConnection(h http.Header) {
	h.Set("Connection", "Upgrade")
}

// VerifyConnection returns an error if the Connection header
// does not contain the case-insensitive "Upgrade" token.
func VerifyConnection(h http.Header) error {
	return httpheaders.VerifyContainsToken(h, "Connection", "Upgrade")
}

// SetUpgrade sets the Upgrade header to "WebSocket".
func SetUpgrade(h http.Header) {
	h.Set("Upgrade", "WebSocket")
}

// VerifyClientUpgrade returns an error if the Upgrade header
// does not contain the "WebSocket" token.
func VerifyClientUpgrade(h http.Header) error {
	return httpheaders.VerifyContainsToken(h, "Upgrade", "WebSocket")
}

// VerifyServerUpgrade returns an error if the Upgrade header
// is not the case-insensitive "WebSocket" token.
func VerifyServerUpgrade(h http.Header) error {
	return httpheaders.VerifyIsToken(h, "Upgrade", "WebSocket")
}

// GetVersion gets the version header value. It returns an error if the
// header doesn't exist, has multiple values, or cannot be parsed.
func GetVersion(h http.Header) (byte, error) {
	// Sec-WebSocket-Version-Client = version
	// Sec-WebSocket-Version-Server = 1#version
	// version = DIGIT | (NZDIGIT DIGIT) |
	//           ("1" DIGIT DIGIT) | ("2" DIGIT DIGIT)
	//           ; Limited to 0-255 range, with no leading zeros
	// NZDIGIT =  "1" | "2" | "3" | "4" | "5" | "6" |
	//            "7" | "8" | "9"
	//
	// NB: The client MUST send a single version to the server.
	// If the server doesn't support this version, its response MUST
	// include a list of its supported versions. However, we only
	// support a single version (13) so we don't support getting
	// or setting multiple versions.
	v, err := getOne(h, VersionKey)
	if err != nil {
		return 0, err
	}
	i, err := strconv.Atoi(v)
	if err != nil || i < 0 || i > 255 {
		return 0, fmt.Errorf("invalid "+VersionKey+" header: value %q is not in range 0-255", v)
	}
	return byte(i), nil
}

// SetVersion sets the version header to the given value.
func SetVersion(h http.Header, version byte) {
	h.Set(VersionKey, strconv.Itoa(int(version)))
}

func getOne(h http.Header, key string) (string, error) {
	switch vals := h.Values(key); len(vals) {
	case 0:
		return "", fmt.Errorf("missing %s header", key)
	case 1:
		if vals[0] == "" {
			return "", fmt.Errorf("invalid %s header: empty value", key)
		}
		return strings.TrimSpace(vals[0]), nil
	default:
		return "", fmt.Errorf("invalid %s header: found multiple values", key)
	}
}
