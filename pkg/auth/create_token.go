package auth

import (
	"fmt"
	"os"

	"github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func MakeToken(userid uuid.UUID, role domain.Role) (string, error) {
	keystr, ok := os.LookupEnv("JWT_SECRET")
	if !ok {
		return "", fmt.Errorf("no jwt secret found")
	}

	key := []byte(keystr)

	t := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			UserId: userid.String(),
			Role:   role,
		})

	s, err := t.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("signing key: %w", err)
	}

	return s, nil
}
