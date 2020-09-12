package wsheaders

import (
	"fmt"
	"net/http"
	"strings"

	"nhooyr.io/websocket/internal/httpheaders"
)

// ProtocolKey is the canonical websocket protocol header key.
const ProtocolKey = "Sec-WebSocket-Protocol"

// Protocols is a list of protocols.
type Protocols []string

func (p Protocols) String() string {
	return strings.Join(p, ", ")
}

// ContainsProtocol returns a bool indicating if the case-insensitive
// protocol exists in the list of protocols.
func ContainsProtocol(protocols Protocols, protocol string) bool {
	return httpheaders.ContainsToken(httpheaders.Tokens(protocols), protocol)
}

// https://tools.ietf.org/html/rfc6455#section-4.3
//
// Sec-WebSocket-Protocol-Client = 1#token
// Sec-WebSocket-Protocol-Server = token

// SetProtocols sets the protocol header to the protocols.
func SetProtocols(header http.Header, protocols ...string) {
	header.Set(ProtocolKey, strings.Join(protocols, ", "))
}

// ParseProtocols parses the protocol values in the given header.
func ParseProtocols(header http.Header) (Protocols, error) {
	values := header.Values(ProtocolKey)
	protos, err := httpheaders.ParseTokenLists(values)
	if err != nil {
		return nil, fmt.Errorf("invalid "+ProtocolKey+" header: %v", err)
	}
	return Protocols(protos), nil
}

// SelectProtocol returns the first case-insensitive supported protocol that
// is offered in the header and a bool indicating if any match was found.
// Priority is given to the order of supported protocols over the header.
func SelectProtocol(header http.Header, supported Protocols) (string, bool) {
	// NB: RFC 6455 specifies that the client should provide its protocols
	// in the header ordered by preference. It does not require the server
	// to accept the client's preference. It only requires that the server
	// accepts one or none of the protocols.
	//
	// This function preserves the existing behavior and gives preference
	// to the order of supported protocols provided by the server.
	offered, err := ParseProtocols(header)
	if err != nil {
		return "", false
	}
	for _, s := range supported {
		if ContainsProtocol(offered, s) {
			return s, true
		}
	}
	return "", false
}

// VerifyProtocol returns the supported protocol that is offered in the header.
// It returns an error if the header is invalid, more than one protocol is offered,
// or it is not supported. It is not an error if no protocols are offered.
func VerifyProtocol(header http.Header, supported Protocols) (string, error) {
	offered, err := ParseProtocols(header)
	if err != nil {
		return "", err
	}
	if len(offered) == 0 {
		return "", nil
	}
	if len(offered) > 1 {
		return "", fmt.Errorf("invalid "+ProtocolKey+" header: %q", offered)
	}
	offer := offered[0]
	for _, s := range supported {
		if strings.EqualFold(s, offer) {
			return s, nil
		}
	}
	return "", fmt.Errorf("unexpected "+ProtocolKey+" header: %q", offered)
}
