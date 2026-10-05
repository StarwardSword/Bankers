package pages

import (
	"context"
	"net/http"

	"github.com/StarwardSword/bank/features/website/adapters/htmx/pages/accounts"
	authpage "github.com/StarwardSword/bank/features/website/adapters/htmx/pages/auth"
	"github.com/StarwardSword/bank/features/website/application"
	"github.com/StarwardSword/bank/pkg/auth"
	"github.com/gorilla/mux"
)

func Configure(ctx context.Context, public *mux.Router, private *mux.Router, app *application.Application) {
	authpage.Configure(ctx, public, private, app)
	accounts.Configure(ctx, private, app)

	public.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ctx, err := auth.PublicAuth(r)
		if err != nil {
			http.Redirect(w, r, "/auth", http.StatusSeeOther)
			return
		}

		r = r.WithContext(ctx)
		http.Redirect(w, r, "/accounts", http.StatusSeeOther)
	})
}
