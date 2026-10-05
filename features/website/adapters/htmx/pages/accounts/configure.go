package accounts

import (
	"context"
	"fmt"
	"net/http"

	"github.com/StarwardSword/bank/features/website/adapters/htmx/pkg/renderer"
	"github.com/StarwardSword/bank/features/website/application"
	"github.com/StarwardSword/bank/pkg/auth"
	"github.com/gorilla/mux"
)

func Configure(ctx context.Context, private *mux.Router, app *application.Application) {
	private.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		auth, err := auth.Authenticate(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("auth: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		renderer.Render(w, r, app.Account, handleIndex(auth.UserId))
	}).Methods("GET")

	private.HandleFunc("/accounts/transfer", func(w http.ResponseWriter, r *http.Request) {
		auth, err := auth.Authenticate(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("auth: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		renderer.Render(w, r, app.Account, handleTransfer(auth.UserId))
	}).Methods("POST")

	private.HandleFunc("/accounts/transfer/{uid:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", func(w http.ResponseWriter, r *http.Request) {
		auth, err := auth.Authenticate(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("auth: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		renderer.Render(w, r, app.Account, handleIndexTransfer(auth.UserId))
	}).Methods("GET")
}
