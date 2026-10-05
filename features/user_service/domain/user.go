package domain

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrInvalidArgument = errors.New("argument error")
	ErrWrongPassword   = errors.New("wrong password")
)

type User struct {
	ID       uuid.UUID
	Username string
	Password string
	Role     Role
}

func NewUser(hasher PasswordHasher, uname, pass string, role Role) (User, error) {
	switch {
	case uname == "":
		return User{}, fmt.Errorf("%w: username is empty", ErrInvalidArgument)
	case pass == "":
		return User{}, fmt.Errorf("%w: password is empty", ErrInvalidArgument)
	case role != RoleGuest && role != RoleUser && role != RoleAdmin:
		return User{}, fmt.Errorf("%w: wrong 'role' value %d", ErrInvalidArgument, role)
	}

	pass = hasher.Hash(pass)

	return User{
		ID:       uuid.New(),
		Username: uname,
		Password: pass,
		Role:     role,
	}, nil
}

func (u *User) Verify(hasher PasswordHasher, password string) (bool, error) {
	return hasher.Compare(password, u.Password)
}

func (u *User) SetPassword(hasher PasswordHasher, oldPassword, newPassword string) (bool, error) {
	if newPassword == "" {
		return false, fmt.Errorf("%w: password is empty", ErrInvalidArgument)
	}

	if verified, err := u.Verify(hasher, oldPassword); !verified || err != nil {
		if err != nil {
			return false, fmt.Errorf("verifying: %w", err)
		}
		return false, ErrWrongPassword
	}

	newPassword = hasher.Hash(newPassword)
	u.Password = newPassword

	return true, nil
}
