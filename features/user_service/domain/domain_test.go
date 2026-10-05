package domain_test

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"

	"github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/google/uuid"
)

func TestUserCreation(t *testing.T) {
	user, uname, pass, hasher := setup(t)

	if user.ID == uuid.Nil {
		t.Fatalf("user id is not set")
	}

	if user.Username != uname {
		t.Fatalf("username corrupted: expected: '%s'; got: '%s'", uname, user.Username)
	}

	if user.Role != domain.RoleUser {
		t.Fatalf("role = %d; want %d", user.Role, domain.RoleUser)
	}

	if user.Password == pass {
		t.Fatalf("hasher forgot to hash password: %s", user.Password)
	}

	requireVerified(t, user, hasher, pass, true)
}

func TestUserCreationEmptyUsername(t *testing.T) {
	user, err := domain.NewUser(&fakeHasher{}, "", "pass", domain.RoleUser)

	requireInvalidArgument(t, user, err)
}

func TestUserCreationEmptyPassword(t *testing.T) {
	user, err := domain.NewUser(&fakeHasher{}, "uname", "", domain.RoleUser)

	requireInvalidArgument(t, user, err)
}

func TestUserCreationInvalidRole(t *testing.T) {
	for _, role := range []domain.Role{domain.RoleGuest - 1, domain.RoleAdmin + 1} {
		t.Run(fmt.Sprintf("role %d", role), func(t *testing.T) {
			user, err := domain.NewUser(&fakeHasher{}, "uname", "pass", role)

			requireInvalidArgument(t, user, err)
		})
	}
}

func TestUserCreationValidRoles(t *testing.T) {
	for _, role := range []domain.Role{domain.RoleGuest, domain.RoleUser, domain.RoleAdmin} {
		t.Run(fmt.Sprintf("role %d", role), func(t *testing.T) {
			user, err := domain.NewUser(&fakeHasher{}, "uname", "pass", role)
			if err != nil {
				t.Fatalf("could not create user: %v", err)
			}

			if user.Role != role {
				t.Fatalf("role = %d; want %d", user.Role, role)
			}
		})
	}
}

func TestPasswordVerification(t *testing.T) {
	user, _, pass, hasher := setup(t)
	wrongPass := "meow meow meow"

	requireVerified(t, user, hasher, pass, true)
	requireVerified(t, user, hasher, wrongPass, false)
}

func TestPasswordVerificationHasherError(t *testing.T) {
	user, _, pass, hasher := setup(t)
	hasher.err = domain.ErrHashService

	_, err := user.Verify(hasher, pass)
	if !errors.Is(err, domain.ErrHashService) {
		t.Fatalf("err = %v; want %v", err, domain.ErrHashService)
	}
}

func TestPasswordUpdate(t *testing.T) {
	user, _, pass, hasher := setup(t)
	newPass := "meow meow meow"

	updated, err := user.SetPassword(hasher, pass, newPass)
	if err != nil || !updated {
		t.Fatalf("SetPassword = %t, %v; want true, nil", updated, err)
	}

	requireVerified(t, user, hasher, newPass, true)
	requireVerified(t, user, hasher, pass, false)
}

func TestPasswordUpdateWrongOldPassword(t *testing.T) {
	user, _, _, hasher := setup(t)
	before := user.Password

	updated, err := user.SetPassword(hasher, "meow meow meow", "new password")
	if !errors.Is(err, domain.ErrWrongPassword) || updated {
		t.Fatalf("SetPassword = %t, %v; want false, %s", updated, err, domain.ErrWrongPassword)
	}

	requirePasswordUnchanged(t, user, before)
}

func TestPasswordUpdateEmptyPassword(t *testing.T) {
	user, _, pass, hasher := setup(t)
	before := user.Password

	updated, err := user.SetPassword(hasher, pass, "")
	if !errors.Is(err, domain.ErrInvalidArgument) || updated {
		t.Fatalf("SetPassword = %t, %v; want false, %v", updated, err, domain.ErrInvalidArgument)
	}

	requirePasswordUnchanged(t, user, before)
}

func TestPasswordUpdateHasherError(t *testing.T) {
	user, _, pass, hasher := setup(t)
	before := user.Password
	hasher.err = domain.ErrHashService

	updated, err := user.SetPassword(hasher, pass, "new password")
	if !errors.Is(err, domain.ErrHashService) || updated {
		t.Fatalf("SetPassword = %t, %v; want false, %v", updated, err, domain.ErrHashService)
	}

	requirePasswordUnchanged(t, user, before)
}

func setup(t *testing.T) (user domain.User, uname string, rawPassword string, hasher *fakeHasher) {
	t.Helper()
	hasher = &fakeHasher{}

	uname = "uname"
	rawPassword = "pass"

	user, err := domain.NewUser(hasher, uname, rawPassword, domain.RoleUser)
	if err != nil {
		t.Fatalf("could not create user: %v", err)
	}

	return user, uname, rawPassword, hasher
}

func requireVerified(t *testing.T, user domain.User, hasher domain.PasswordHasher, password string, want bool) {
	t.Helper()

	got, err := user.Verify(hasher, password)
	if err != nil {
		t.Fatalf("verify %q: %v", password, err)
	}
	if got != want {
		t.Fatalf("verify %q = %t; want %t", password, got, want)
	}
}

func requireInvalidArgument(t *testing.T, user domain.User, err error) {
	t.Helper()

	if !errors.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("err = %v; want %v", err, domain.ErrInvalidArgument)
	}
	if user != (domain.User{}) {
		t.Fatalf("user = %+v; want zero value on error", user)
	}
}

func requirePasswordUnchanged(t *testing.T, user domain.User, before string) {
	t.Helper()

	if user.Password != before {
		t.Fatalf("password changed after failed update")
	}
}

type fakeHasher struct {
	err error
}

func (h *fakeHasher) Hash(password string) string {
	sum := sha256.Sum256([]byte(password))

	return string(sum[:])
}

func (h *fakeHasher) Compare(raw, hashed string) (bool, error) {
	if h.err != nil {
		return false, h.err
	}

	return h.Hash(raw) == hashed, nil
}
