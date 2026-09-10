package errors

import (
	"fmt"
	"net/http"
)

const (
	KIND_FQN_SEPARATOR      = "::"
	HTTP_STATUS_NOT_DEFINED = 0
)

var (
	registeredCodes = map[string]*Kind{}
	registeredNames = map[string]*Kind{}
)

type Kind struct {
	Name           string
	Code           string
	Description    string
	HTTPStatusCode int
	Parent         *Kind

	// customStatus marks a value produced by CustomizeStatusCode, which is how
	// propagation tells a status pinned on purpose from a kind's own default.
	customStatus bool
}

// Register adds kinds to the global registry. It panics on a duplicate code or
// name, because the registry is what decodes a kind received over the wire and
// two kinds answering to one identifier would make that ambiguous.
func Register(kinds ...*Kind) {
	for _, kind := range kinds {
		if _, ok := registeredCodes[kind.Code]; ok {
			panic("another kind with the same code was already registered")
		}

		if _, ok := registeredNames[kind.Name]; ok {
			panic("another kind with the same name was already registered")
		}

		registeredCodes[kind.Code] = kind
		registeredNames[kind.Name] = kind
	}
}

// GetByCode returns the registered Kind with the given code and panics when the
// code is unknown. Use LookupByCode for a code that came from outside the
// program.
func GetByCode(code string) *Kind {
	kind, ok := LookupByCode(code)
	if !ok {
		panic(fmt.Errorf("error kind with code `%s` doesn't exist", code))
	}

	return kind
}

// LookupByCode retrieves a registered Kind by its code without panicking. It
// returns (kind, true) when the code is registered and (nil, false) otherwise.
// Use it for untrusted input, such as a kind code decoded from a response body
// received over the wire; use GetByCode for internal codes the program controls.
func LookupByCode(code string) (*Kind, bool) {
	kind, ok := registeredCodes[code]
	return kind, ok
}

// Get returns the registered Kind with the given name and panics when the name
// is unknown.
func Get(name string) *Kind {
	kind, ok := registeredNames[name]
	if !ok {
		panic(fmt.Errorf("error kind with name `%s` doesn't exist", name))
	}

	return kind
}

// FQN spells the kind's ancestry, from its own name up to the root, joined by
// KIND_FQN_SEPARATOR.
func (k *Kind) FQN() string {
	if k.Parent == nil {
		return k.Name
	}

	return fmt.Sprintf("%s%s%s", k.Name, KIND_FQN_SEPARATOR, k.Parent.FQN())
}

// StatusCode returns the first HTTP status found walking up the hierarchy, so a
// kind can leave the status to its parent. It falls back to 500 when no ancestor
// defines one.
func (k *Kind) StatusCode() int {
	for current := k; current != nil; current = current.Parent {
		if current.HTTPStatusCode != HTTP_STATUS_NOT_DEFINED {
			return current.HTTPStatusCode
		}
	}

	return http.StatusInternalServerError
}

// CustomizeStatusCode returns a copy of the kind that reports code as its HTTP
// status. It is what a status received from elsewhere needs: [KindForStatus] can
// only classify a non-standard 499 or 599 as its 4xx or 5xx class, so the exact
// code would otherwise be lost.
//
// The copy keeps the name, code, description, and parent of the original, so its
// FQN, its kind matching, and the DTO it produces are the ones the original
// would produce; only the status differs. Neither the receiver nor the registry
// is touched, so a registered kind is safe to customize repeatedly and from
// several goroutines. A non-positive code is ignored and returns the receiver.
func (k *Kind) CustomizeStatusCode(code int) *Kind {
	if code <= 0 {
		return k
	}

	custom := *k
	custom.HTTPStatusCode = code
	custom.customStatus = true

	return &custom
}

// Is reports whether the kind or any of its ancestors carries the code of other,
// so matching a parent kind also matches everything below it. Codes are compared
// rather than pointers, because a customized kind is a copy.
func (k *Kind) Is(other *Kind) bool {
	for current := k; current != nil; current = current.Parent {
		if current.Code == other.Code {
			return true
		}
	}
	return false
}

// KindCoder is implemented by errors that expose a bare kind code, such as a
// headless origin reference decoded from the wire. It lets callers read the code
// without knowing the concrete error type.
type KindCoder interface {
	Code() string
}

// CodeOf returns the kind code carried by err, or an empty string when err does
// not expose one.
func CodeOf(err error) string {
	e, ok := err.(KindCoder)
	if !ok {
		return ""
	}

	return e.Code()
}
