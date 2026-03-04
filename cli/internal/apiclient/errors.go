package apiclient

import "errors"

// ErrNotFound is returned when the API responds with 404.
var ErrNotFound = errors.New("not found")

// ErrUnauthorized is returned when the API responds with 401 or 403.
var ErrUnauthorized = errors.New("unauthorized; run 'pkgtool auth login'")
