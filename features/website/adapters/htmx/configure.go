package htmx

import (
	"context"
	"fmt"
	"net/http"

	"github.com/StarwardSword/bank/features/website/adapters/htmx/pages"
	"github.com/StarwardSword/bank/features/website/application"
	"github.com/StarwardSword/bank/features/website/application/accountservice"
	"github.com/StarwardSword/bank/features/website/application/authservice"
	"github.com/StarwardSword/bank/features/website/application/userservice"
	"github.com/gorilla/mux"
)

type Services struct {
	Auth    authservice.Service
	Account accountservice.Service
	Users   userservice.Service
}

func Configure(ctx context.Context, public *mux.Router, private *mux.Router, services *Services) error {
	public.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./features/website/adapters/htmx/static/"))))
	app, err := application.NewApplication(services.Auth, services.Account, services.Users)
	if err != nil {
		return fmt.Errorf("creating app: %w", err)
	}

	pages.Configure(ctx, public, private, app)
	return nil
}
