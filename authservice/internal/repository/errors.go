package repository

import "errors"

var (
	ErrorBadRequest          = errors.New("bad_request")
	ErrorInternalServerError = errors.New("internal_server_error")
	ErrorConflict            = errors.New("data_conflict")
)
