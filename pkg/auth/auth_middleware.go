package auth

import (
	"context"
	"fmt"
	"net/http"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := parse(r)
		if err != nil {
			http.Error(w, fmt.Sprintf("auth: %s", err.Error()), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func PublicAuth(r *http.Request) (context.Context, error) {
	return parse(r)
}
