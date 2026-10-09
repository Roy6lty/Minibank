package repository

import (
	"context"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type UserRepository interface {
	Create(context.Context, db.CreateUserParams) (db.User, error)
	GetByEmail(context.Context, string) (db.User, error)
	GetByID(context.Context, pgtype.UUID) (db.User, error)
}

type RefreshTokenRepository interface {
	Create(context.Context, db.CreateRefreshTokenParams) (db.RefreshToken, error)
	GetByHash(context.Context, string) (db.RefreshToken, error)
	Revoke(context.Context, string) error
}

type userRepository struct {
	queries *db.Queries
}

type refreshTokenRepository struct {
	queries *db.Queries
}

func NewUserRepository(queries *db.Queries) UserRepository {
	return &userRepository{queries: queries}
}

func NewRefreshTokenRepository(queries *db.Queries) RefreshTokenRepository {
	return &refreshTokenRepository{queries: queries}
}

func (r *userRepository) Create(
	ctx context.Context,
	arg db.CreateUserParams,
) (db.User, error) {
	return r.queries.CreateUser(ctx, arg)
}

func (r *userRepository) GetByEmail(
	ctx context.Context,
	email string,
) (db.User, error) {
	return r.queries.GetUserByEmail(ctx, email)
}

func (r *userRepository) GetByID(
	ctx context.Context,
	id pgtype.UUID,
) (db.User, error) {
	return r.queries.GetUserByID(ctx, id)
}

func (r *refreshTokenRepository) Create(
	ctx context.Context,
	arg db.CreateRefreshTokenParams,
) (db.RefreshToken, error) {
	return r.queries.CreateRefreshToken(ctx, arg)
}

func (r *refreshTokenRepository) GetByHash(
	ctx context.Context,
	tokenHash string,
) (db.RefreshToken, error) {
	return r.queries.GetRefreshTokenByHash(ctx, tokenHash)
}

func (r *refreshTokenRepository) Revoke(
	ctx context.Context,
	tokenHash string,
) error {
	return r.queries.RevokeRefreshToken(ctx, tokenHash)
}
