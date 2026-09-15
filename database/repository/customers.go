package repository

import (
	"context"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type CustomerRepository interface {
	Create(
		ctx context.Context,
		arg db.CreateCustomerParams,
	) (db.Customer, error)

	GetByID(
		ctx context.Context,
		id pgtype.UUID,
	) (db.Customer, error)

	GetByEmail(
		ctx context.Context,
		email string,
	) (db.Customer, error)

	GetCustomersList(
		ctx context.Context,
		arg db.GetCustomersParams,
	) ([]db.Customer, error)

	SearchByName(
		ctx context.Context,
		arg db.SearchCustomersByNameParams,
	) ([]db.Customer, error)

	UpdateByID(
		ctx context.Context,
		arg db.UpdateCustomerByIDParams,
	) (db.Customer, error)

	List(
		ctx context.Context,
		sort CustomerSort,
		limit int32,
		offset int32,
	) ([]db.Customer, error)
}

type CustomerSort string

const (
	SortByCreatedAt CustomerSort = "created_at"
	SortByFirstName CustomerSort = "first_name"
	SortByLastName  CustomerSort = "last_name"
)

type customerRepository struct {
	queries *db.Queries
}

func NewCustomerRepository(queries *db.Queries) CustomerRepository {
	return &customerRepository{queries: queries}
}

func (r *customerRepository) Create(
	ctx context.Context,
	arg db.CreateCustomerParams,
) (db.Customer, error) {
	return r.queries.CreateCustomer(ctx, arg)
}

func (r *customerRepository) GetByID(
	ctx context.Context,
	id pgtype.UUID,
) (db.Customer, error) {
	return r.queries.GetCustomer(ctx, id)
}

func (r *customerRepository) GetByEmail(
	ctx context.Context,
	email string,
) (db.Customer, error) {
	return r.queries.GetCustomerByEmail(ctx, email)
}

func (r *customerRepository) GetCustomersList(
	ctx context.Context,
	arg db.GetCustomersParams,
) ([]db.Customer, error) {
	return r.queries.GetCustomers(ctx, arg)
}

func (r *customerRepository) SearchByName(
	ctx context.Context,
	arg db.SearchCustomersByNameParams,
) ([]db.Customer, error) {
	return r.queries.SearchCustomersByName(ctx, arg)
}

func (r *customerRepository) UpdateByID(
	ctx context.Context,
	arg db.UpdateCustomerByIDParams,
) (db.Customer, error) {
	return r.queries.UpdateCustomerByID(ctx, arg)
}

func (r *customerRepository) List(
	ctx context.Context,
	sort CustomerSort,
	limit int32,
	offset int32,
) ([]db.Customer, error) {

	switch sort {
	case SortByFirstName:
		return r.queries.GetCustomersOrderedByFirstName(
			ctx,
			db.GetCustomersOrderedByFirstNameParams{
				Limit:  limit,
				Offset: offset,
			},
		)

	case SortByLastName:
		return r.queries.GetCustomersOrderedByLastName(
			ctx,
			db.GetCustomersOrderedByLastNameParams{
				Limit:  limit,
				Offset: offset,
			},
		)

	default:
		return r.queries.GetCustomersByCreatedAt(
			ctx,
			db.GetCustomersByCreatedAtParams{
				Limit:  limit,
				Offset: offset,
			},
		)
	}
}
