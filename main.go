package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	accountppLocal "github.com/StarwardSword/bank/features/accounts/adapters/local"
	ledgerappLocal "github.com/StarwardSword/bank/features/ledger/adapters/local"
	userappLocal "github.com/StarwardSword/bank/features/user_service/adapters/local"
	"github.com/StarwardSword/bank/features/website/adapters/htmx"
	"github.com/StarwardSword/bank/features/website/infrastucture/localauthservice"
	"github.com/StarwardSword/bank/pkg/auth"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var addr = flag.String("address", "localhost:8085", "specify address to serve")

func main() {
	flag.Parse()
	mux := mux.NewRouter()
	ctx := context.Background()

	err := godotenv.Load()
	if err != nil {
		panic(fmt.Errorf("can not load .env: %w", err))
	}

	if secret, ok := os.LookupEnv("JWT_SECRET"); !ok || secret == "" {
		panic("JWT_SECRET is not set")
	}

	requiresAuth := mux.NewRoute().Subrouter()
	requiresAuth.Use(auth.Middleware)

	pool, err := pgxpool.New(ctx, os.Getenv("DB_CONNECTION_STRING"))
	if err != nil {
		panic(fmt.Sprintf("creating pool: %s", err.Error()))
	}

	userapp, err := userappLocal.Configure(ctx, pool)
	if err != nil {
		panic(fmt.Sprintf("creating user app: %s", err.Error()))
	}

	ledger, err := ledgerappLocal.Configure(pool)
	if err != nil {
		panic(fmt.Sprintf("creating ledger app: %s", err.Error()))
	}

	accountapp, err := accountppLocal.Configure(ctx, pool, &accountLedgerApp{app: ledger})
	if err != nil {
		panic(fmt.Sprintf("creating account app: %s", err.Error()))
	}

	htmxAuth, err := localauthservice.NewService(userapp)
	if err != nil {
		panic(fmt.Sprintf("creating auth service: %s", err.Error()))
	}

	err = htmx.Configure(ctx, mux, requiresAuth, &htmx.Services{
		Auth:    &htmxAuthApp{auth: htmxAuth},
		Account: &htmxAccountApp{accounts: accountapp},
		Users:   &htmxUserApp{app: userapp},
	})
	if err != nil {
		panic(fmt.Sprintf("creating htmx service: %s", err.Error()))
	}

	println("Up and running at ", *addr)
	for {
		err = http.ListenAndServe(*addr, mux)
		if err != nil {
			println(err.Error())
			time.Sleep(1 * time.Second)
			println("Restarting...")
		}
	}
}
