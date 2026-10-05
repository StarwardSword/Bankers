package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/StarwardSword/bank/features/website/adapters/htmx/pkg/renderer"
	"github.com/StarwardSword/bank/features/website/application/authservice"
	"github.com/StarwardSword/bank/features/website/application/userservice"
	"github.com/a-h/templ"
)

type registerQuery struct {
	Username string `form:"username"`
	Password string `form:"password"`
}

type registrationServices struct {
	UserService userservice.Service
	AuthService authservice.Service
}

func handleRegistration(ptr *string) renderer.Handler[registrationServices, *registerQuery] {
	return func(ctx context.Context, service registrationServices, q *registerQuery) (templ.Component, error) {
		q.Username = strings.TrimSpace(q.Username)

		if q.Username == "" {
			return usernameContainerError(q.Username, "Обязательное поле"), nil
		}
		if q.Password == "" {
			return passwordContainerError(q.Password, "Обязательное поле"), nil
		}

		_, err := service.UserService.CreateUser(ctx, q.Username, q.Password)
		switch {
		case errors.Is(err, userservice.ErrAlreadyExists):
			return usernameContainerError(q.Username, "Пользователь с таким именем уже сущетсвует"), nil
		case errors.Is(err, userservice.ErrInvalidArgument):
			return usernameContainerError(q.Username, "Обязательное поле"), nil
		case err != nil:
			return nil, fmt.Errorf("can not create user: %w", err)
		}

		tkn, err := service.AuthService.Auth(ctx, q.Username, q.Password)
		if errors.Is(err, authservice.ErrNotFound) || errors.Is(err, authservice.ErrWrongPassword) {
			return passwordContainerError(q.Password, "Если вы это видите, то что-то ужасно сломано"), fmt.Errorf("can not auth freshly created user: %w", err)
		}
		if err != nil {
			return nil, fmt.Errorf("authenticating: %w", err)
		}

		*ptr = tkn

		return accountRedirect(), nil
	}
}
