package application

import "errors"

var (
	ErrAlreadyExists    = errors.New("user already exists")
	ErrNotFound         = errors.New("not found")
	ErrWrongPassword    = errors.New("wrong password")
	ErrPermissionDenied = errors.New("forbidden")
)
