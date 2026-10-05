package passwordhasher

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"

	"github.com/StarwardSword/bank/features/user_service/domain"
	"golang.org/x/crypto/argon2"
)

var _ domain.PasswordHasher = new(SimpleArgon2Hasher)

type argonParams struct {
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
}

type SimpleArgon2Hasher struct {
	saltSize int
	params   argonParams
}

func NewSimpleArgon2Hasher() *SimpleArgon2Hasher {
	return &SimpleArgon2Hasher{
		saltSize: 32,
		params: argonParams{
			Time:    1,
			Memory:  64 * 1024,
			Threads: 4,
			KeyLen:  32,
		},
	}
}

func (h *SimpleArgon2Hasher) Hash(password string) string {
	salt := make([]byte, h.saltSize)
	_, _ = rand.Read(salt)

	hash := argon2.IDKey([]byte(password), salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen)
	hash = append(hash, salt...)

	return base64.StdEncoding.EncodeToString(hash)
}

func (h *SimpleArgon2Hasher) Compare(raw, hashed string) (bool, error) {
	hash, err := base64.StdEncoding.DecodeString(hashed)
	if err != nil {
		return false, fmt.Errorf("decoding string: %s: %w", err.Error(), domain.ErrHashService)
	}

	salt := hash[h.params.KeyLen:]
	hash = hash[:h.params.KeyLen]
	secondHash := argon2.IDKey([]byte(raw), salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen)

	return subtle.ConstantTimeCompare(hash, secondHash) == 1, nil
}
