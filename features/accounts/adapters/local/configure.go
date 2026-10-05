package local

import (
	"context"
	"fmt"

	"github.com/StarwardSword/bank/features/accounts/application"
	"github.com/StarwardSword/bank/features/accounts/domain"
	"github.com/StarwardSword/bank/features/accounts/infrastructure/postgrex"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Configure(ctx context.Context, pool *pgxpool.Pool, ldr domain.Ledger) (*application.Application, error) {
	err := pool.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not connect to database: %w", err)
	}

	repo, err := postgrex.NewRepository(pool)
	if err != nil {
		return nil, fmt.Errorf("could not create repository: %w", err)
	}

	app, err := application.NewApplication(repo, ldr)
	if err != nil {
		return nil, fmt.Errorf("could not create applicaion: %w", err)
	}

	return app, nil
}
