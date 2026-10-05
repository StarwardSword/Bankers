package domain

import "errors"

var (
	ErrHashService = errors.New("internal hash service error")
)

type PasswordHasher interface {
	Hash(password string) string
	Compare(raw, hashed string) (bool, error)
}
