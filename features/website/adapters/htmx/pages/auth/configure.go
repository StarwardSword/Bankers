package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/StarwardSword/bank/features/website/adapters/htmx/pkg/renderer"
	"github.com/StarwardSword/bank/features/website/application"
	"github.com/StarwardSword/bank/pkg/auth"
	"github.com/gorilla/mux"
)

func setCookie(w http.ResponseWriter, tkn string) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.AccessTokenCookie,
		Value:    tkn,
		HttpOnly: true,
		// WARN: Set it to true when you get https
		Secure: false,
		Path:   "/",
		MaxAge: 0,
	})
}

func Configure(ctx context.Context, public *mux.Router, private *mux.Router, app *application.Application) {
	public.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		renderer.Render(w, r, nil, handle)
	}).Methods("GET")

	public.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		services := registrationServices{UserService: app.Users, AuthService: app.Auth}
		tkn := ""

		renderer.RenderWithCookie(w, r, services, handleRegistration(&tkn), func(w http.ResponseWriter) error {
			if tkn == "" {
				return fmt.Errorf("nil token")
			}

			setCookie(w, tkn)

			return nil
		})
	}).Methods("POST")

	public.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		tkn := ""
		handler := handleAuth(&tkn)

		// http.Redirect(w, r, "/accounts", http.StatusSeeOther)
		renderer.RenderWithCookie(w, r, app.Auth, handler, func(w http.ResponseWriter) error {
			if tkn == "" {
				return nil
			}

			setCookie(w, tkn)

			return nil
		})
	}).Methods("POST")

	private.HandleFunc("/auth/probe", func(w http.ResponseWriter, r *http.Request) {
		auth, err := auth.Authenticate(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("auth error: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "userid: %d, role: %d", auth.UserId, auth.Role)
	}).Methods("GET")

	public.HandleFunc("/auth/switch/register", func(w http.ResponseWriter, r *http.Request) {
		renderer.Render(w, r, nil, handleSwitch(false))
	}).Methods("GET")

	public.HandleFunc("/auth/switch/signin", func(w http.ResponseWriter, r *http.Request) {
		renderer.Render(w, r, nil, handleSwitch(true))
	}).Methods("GET")
}
