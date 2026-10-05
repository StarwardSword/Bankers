package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/StarwardSword/bank/features/website/adapters/htmx/pkg/renderer"
	"github.com/StarwardSword/bank/features/website/application/authservice"
	"github.com/a-h/templ"
)

func handle(ctx context.Context, service any, query any) (templ.Component, error) {
	return index(), nil
}

type authQuery struct {
	Username string `form:"username"`
	Password string `form:"password"`
}

func handleAuth(tokenptr *string) renderer.Handler[authservice.Service, *authQuery] {
	return func(ctx context.Context, authService authservice.Service, q *authQuery) (templ.Component, error) {
		q.Username = strings.TrimSpace(q.Username)

		if q.Username == "" {
			return usernameContainerError(q.Username, "Обязательное поле"), nil
		}
		if q.Password == "" {
			return passwordContainerError(q.Password, "Обязательное поле"), nil
		}

		tkn, err := authService.Auth(ctx, q.Username, q.Password)
		if errors.Is(err, authservice.ErrNotFound) || errors.Is(err, authservice.ErrWrongPassword) {
			return passwordContainerError(q.Password, "Неправильное имя пользователя или пароль"), nil
		}
		if err != nil {
			return nil, fmt.Errorf("running auth: %w", err)
		}

		*tokenptr = tkn

		return accountRedirect(), nil
	}
}
