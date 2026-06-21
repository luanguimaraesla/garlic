package errors

import "net/http"

// Garlic groups error kinds into three tiers, distinguished by a code prefix:
//
//   - Primitive kinds ("P"): the abstract roots of the hierarchy.
//   - Secondary kinds ("S"): one per standard HTTP status (see status.go).
//   - Tertiary kinds ("C"): framework-specific errors that descend from the
//     primitive and secondary kinds.
//
// The P, S, and C prefixes are reserved by garlic; custom kinds must use a
// different prefix.
var (
	// Primitive kinds (P).

	KindError = &Kind{
		Name:           "Error",
		Code:           "P00000",
		Description:    "Any error that has not been mapped in the application.",
		HTTPStatusCode: HTTP_STATUS_NOT_DEFINED,
		Parent:         nil,
	}

	KindUserError = &Kind{
		Name:           "UserError",
		Code:           "P00001",
		Description:    "Any error that was caused by some incorrect user action.",
		HTTPStatusCode: http.StatusBadRequest,
		Parent:         KindError,
	}

	KindSystemError = &Kind{
		Name:           "SystemError",
		Code:           "P00002",
		Description:    "Any error that was caused by some unexpected system failure.",
		HTTPStatusCode: http.StatusInternalServerError,
		Parent:         KindError,
	}

	// Tertiary kinds (C).

	KindInvalidRequestError = &Kind{
		Name:        "InvalidRequestError",
		Code:        "C00001",
		Description: "The request is incorrectly formatted or has errors in the request body.",
		Parent:      httpKinds[http.StatusBadRequest],
	}

	KindAuthError = &Kind{
		Name:        "AuthError",
		Code:        "C00003",
		Description: "An error occurred during authentication, such as invalid credentials.",
		Parent:      httpKinds[http.StatusUnauthorized],
	}

	KindForbiddenError = &Kind{
		Name:        "ForbiddenError",
		Code:        "C00004",
		Description: "The user does not have permission to access the requested resource.",
		Parent:      httpKinds[http.StatusForbidden],
	}

	KindNotFoundError = &Kind{
		Name:        "NotFoundError",
		Code:        "C00005",
		Description: "The requested resource was not found in our system or external services.",
		Parent:      httpKinds[http.StatusNotFound],
	}

	KindContextError = &Kind{
		Name:        "ContextError",
		Code:        "C00006",
		Description: "An error occurred due to a problem with the context.",
		Parent:      KindSystemError,
	}

	KindContextValueNotFoundError = &Kind{
		Name:        "ContextValueNotFoundError",
		Code:        "C00007",
		Description: "A required value was not found in the context.",
		Parent:      KindContextError,
	}
)

func init() {
	// Primitive kinds (P).
	Register(
		KindError,
		KindUserError,
		KindSystemError,
	)

	// Secondary kinds (S), one per standard HTTP status.
	registerHTTPKinds()

	// Tertiary kinds (C).
	Register(
		KindInvalidRequestError,
		KindAuthError,
		KindForbiddenError,
		KindNotFoundError,
		KindContextError,
		KindContextValueNotFoundError,
	)
}
