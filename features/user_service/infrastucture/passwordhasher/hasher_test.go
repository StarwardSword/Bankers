package passwordhasher_test

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/StarwardSword/bank/features/user_service/infrastucture/passwordhasher"
)

const (
	keyLen  = 32
	saltLen = 32

	knownPassword = "password"
	knownHash     = "Kvzf8w0HX0rz126JwqTVr6xW8iXBSH62GRQ3it6XX/ZciV046bv932Vc3Iw6og6Wrr07/FyLUrOP0lFty7/RUw=="
)

func requireCompare(t *testing.T, hasher domain.PasswordHasher, raw, hashed string, want bool) {
	t.Helper()

	got, err := hasher.Compare(raw, hashed)
	if err != nil || got != want {
		t.Fatalf("Compare(%q) = %t, %v; want %t, nil", raw, got, err, want)
	}
}

func TestHasherRoundTrip(t *testing.T) {
	hasher := passwordhasher.NewSimpleArgon2Hasher()

	cases := []struct {
		name     string
		password string
	}{
		{"ascii", "password"},
		{"unicode", "пароль"},
		{"spaces", " password with spaces "},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hashed := hasher.Hash(c.password)

			requireCompare(t, hasher, c.password, hashed, true)
		})
	}
}

func TestHasherWrongPassword(t *testing.T) {
	hasher := passwordhasher.NewSimpleArgon2Hasher()
	hashed := hasher.Hash("password")

	for _, wrong := range []string{"Password", "password ", "passwor", ""} {
		requireCompare(t, hasher, wrong, hashed, false)
	}
}

func TestHasherSaltsEveryHash(t *testing.T) {
	hasher := passwordhasher.NewSimpleArgon2Hasher()

	first := hasher.Hash("password")
	second := hasher.Hash("password")
	if first == second {
		t.Fatalf("two hashes of the same password are equal: %s", first)
	}

	requireCompare(t, hasher, "password", first, true)
	requireCompare(t, hasher, "password", second, true)
}

func TestHasherHashFormat(t *testing.T) {
	hasher := passwordhasher.NewSimpleArgon2Hasher()

	decoded, err := base64.StdEncoding.DecodeString(hasher.Hash("password"))
	if err != nil {
		t.Fatalf("hash is not standard base64: %v", err)
	}
	if len(decoded) != keyLen+saltLen {
		t.Fatalf("decoded hash has %d byte(s); want %d of key and %d of salt", len(decoded), keyLen, saltLen)
	}
}

func TestHasherKnownHash(t *testing.T) {
	hasher := passwordhasher.NewSimpleArgon2Hasher()

	requireCompare(t, hasher, knownPassword, knownHash, true)
	requireCompare(t, hasher, "wrong password", knownHash, false)
}

func TestHasherTamperedHash(t *testing.T) {
	hasher := passwordhasher.NewSimpleArgon2Hasher()

	cases := []struct {
		name  string
		index int
	}{
		{"key", 0},
		{"salt", keyLen},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			decoded, err := base64.StdEncoding.DecodeString(knownHash)
			if err != nil {
				t.Fatalf("decoding known hash: %v", err)
			}
			decoded[c.index] ^= 0xff

			requireCompare(t, hasher, knownPassword, base64.StdEncoding.EncodeToString(decoded), false)
		})
	}
}

func TestHasherInvalidBase64(t *testing.T) {
	hasher := passwordhasher.NewSimpleArgon2Hasher()

	ok, err := hasher.Compare(knownPassword, "not base64!")
	if ok || !errors.Is(err, domain.ErrHashService) {
		t.Fatalf("Compare = %t, %v; want false, %v", ok, err, domain.ErrHashService)
	}
}
