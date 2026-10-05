package local

import (
	"context"
	"fmt"

	"github.com/StarwardSword/bank/features/user_service/application"
	"github.com/StarwardSword/bank/features/user_service/domain"
	"github.com/StarwardSword/bank/features/user_service/infrastucture/passwordhasher"
	"github.com/StarwardSword/bank/features/user_service/infrastucture/postgrex"
	"github.com/StarwardSword/bank/pkg/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Configure(ctx context.Context, pool *pgxpool.Pool) (*application.Application, error) {
	err := pool.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not connect to database: %w", err)
	}

	repo := postgrex.NewRepository(ctx, pool)
	hasher := passwordhasher.NewSimpleArgon2Hasher()

	app := application.NewApplication(hasher, repo, &authService{})
	return app, nil
}

type authService struct{}

func (a *authService) MakeToken(id uuid.UUID, role domain.Role) (string, error) {
	return auth.MakeToken(id, role)
}
