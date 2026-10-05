package authservice

import (
	"context"
	"errors"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrWrongPassword = errors.New("wrong password")
)

type Service interface {
	Auth(ctx context.Context, username string, password string) (string, error)
}
