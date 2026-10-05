package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const UserId = "user_id"
const Role = "role"
const authHeader = "Authorization"
const bearerTag = "beerer"
const bearerSeparator = ": "
const AccessTokenCookie = "assess_token"

type claims struct {
	UserID string `json:"user_id"`
	Role   int    `json:"role"`
	jwt.RegisteredClaims
}

func parse(r *http.Request) (context.Context, error) {
	var tokenstr string = ""

	header := r.Header.Get(authHeader)
	if header != "" {
		parts := strings.Split(header, bearerSeparator)
		if len(parts) == 2 && parts[0] == bearerTag {
			tokenstr = parts[1]
		}
	}

	if tokenstr == "" {
		cookie, err := r.Cookie(AccessTokenCookie)
		if err == nil {
			tokenstr = cookie.Value
		}
	}

	if tokenstr == "" {
		return nil, fmt.Errorf("no authentication header")
	}

	claims := &claims{}
	token, err := jwt.ParseWithClaims(tokenstr, claims, func(t *jwt.Token) (any, error) {
		key, ok := os.LookupEnv("JWT_SECRET")
		if !ok {
			return nil, fmt.Errorf("can not find jwt secret")
		}

		return []byte(key), nil
	})

	if err != nil {
		return nil, fmt.Errorf("error parsing token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid auth token")
	}

	uid, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("could not parse uid: %w", err)
	}

	ctx := context.WithValue(r.Context(), UserId, uid)
	ctx = context.WithValue(ctx, Role, claims.Role)

	return ctx, nil
}
