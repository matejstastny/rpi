package web

import "errors"

var (
	errBadRequest = errors.New("could not read that request")
	errBadJSON    = errors.New("that request body is not valid json")
)
