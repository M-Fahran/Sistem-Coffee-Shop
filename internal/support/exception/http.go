package exception

import "net/http"

// BadRequest returns a 400 error.
func BadRequest(errCode, msg string) *Exception {
	return New(http.StatusBadRequest, errCode, msg)
}

// Unauthorized returns a 401 error.
func Unauthorized(errCode, msg string) *Exception {
	return New(http.StatusUnauthorized, errCode, msg)
}

// Forbidden returns a 403 error.
func Forbidden(errCode, msg string) *Exception {
	return New(http.StatusForbidden, errCode, msg)
}

// NotFound returns a 404 error.
func NotFound(errCode, msg string) *Exception {
	return New(http.StatusNotFound, errCode, msg)
}

// Conflict returns a 409 error (business rule violation).
func Conflict(errCode, msg string) *Exception {
	return New(http.StatusConflict, errCode, msg)
}

// Internal returns a 500 error. Wraps the real error for logging,
// message is always generic.
func Internal(err error) *Exception {
	return Wrap(err, http.StatusInternalServerError, "SYS_500", "internal server error")
}