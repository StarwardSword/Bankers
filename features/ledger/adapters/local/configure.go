package local

import (
	"fmt"

	"github.com/StarwardSword/bank/features/ledger/application"
	"github.com/StarwardSword/bank/features/ledger/infrastructure/postgrex"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Configure(pool *pgxpool.Pool) (*application.Application, error) {
	repo, err := postgrex.NewRepository(pool)
	if err != nil {
		return nil, fmt.Errorf("creating repository: %w", err)
	}

	app, err := application.NewApplication(repo)
	if err != nil {
		return nil, fmt.Errorf("creating application: %w", err)
	}

	return app, nil
}
