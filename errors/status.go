package errors

import (
	"fmt"
	"net/http"
)

// httpKinds maps every standard HTTP status code to its dedicated secondary kind
// (the "S"-prefixed kinds). It is built during package variable initialization,
// before any init() runs, so the tertiary kinds in common.go can parent off these
// entries directly.
var httpKinds = buildHTTPKinds()

// buildHTTPKinds creates one secondary kind for every standard HTTP status, named
// "HTTP<status>Error" with code "S<status>", parented off its class base.
func buildHTTPKinds() map[int]*Kind {
	kinds := map[int]*Kind{}

	for status := 100; status <= 599; status++ {
		text := http.StatusText(status)
		if text == "" {
			continue // not a standard status
		}

		kinds[status] = &Kind{
			Name:           fmt.Sprintf("HTTP%dError", status),
			Code:           fmt.Sprintf("S%05d", status),
			Description:    text,
			HTTPStatusCode: status,
			Parent:         classForStatus(status),
		}
	}

	return kinds
}

// registerHTTPKinds registers every secondary kind with the global registry.
func registerHTTPKinds() {
	for _, kind := range httpKinds {
		Register(kind)
	}
}

// classForStatus returns the primitive class base for an HTTP status: KindUserError
// for 4xx client errors, and KindSystemError for every other status (any status
// below 400 or 500 and above).
func classForStatus(status int) *Kind {
	if status >= 400 && status < 500 {
		return KindUserError
	}
	return KindSystemError
}

// KindForStatus returns the secondary Kind whose StatusCode() equals status. Any
// non-standard status falls back to its primitive class base, so the classification
// and the wire status stay consistent.
func KindForStatus(status int) *Kind {
	if kind, ok := httpKinds[status]; ok {
		return kind
	}
	return classForStatus(status)
}
