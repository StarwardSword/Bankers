package auth

import (
	"context"

	"github.com/StarwardSword/bank/features/website/adapters/htmx/pkg/renderer"
	"github.com/a-h/templ"
)

func handleSwitch(signin bool) renderer.Handler[any, any] {
	return func(ctx context.Context, service any, query any) (templ.Component, error) {
		return SignRegBlock(signin, true), nil
	}
}
