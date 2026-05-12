package target

import (
	"fmt"
	"net/url"
	"strconv"
)

// FromQuery parses downstream host and port from URL query values (keys h and p).
func FromQuery(q url.Values) (host string, port int, err error) {
	host = q.Get("h")
	if host == "" {
		return "", 0, fmt.Errorf("missing required query parameter: h")
	}
	portStr := q.Get("p")
	if portStr == "" {
		return "", 0, fmt.Errorf("missing required query parameter: p")
	}
	p, err := strconv.Atoi(portStr)
	if err != nil || p < 1 || p > 65535 {
		return "", 0, fmt.Errorf("invalid port: %q", portStr)
	}
	return host, p, nil
}
