package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Roy6lty/minibank/database/repository"
	db "github.com/Roy6lty/minibank/internal/sqlc"
	authModels "github.com/Roy6lty/minibank/modules/auth"
	models "github.com/Roy6lty/minibank/modules/customer"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type CustomerService interface {
	CreateCustomer(
		ctx context.Context,
		principal authModels.Principal,
		request models.CreateCustomerRequest,
	) error

	GetCustomerByID(
		ctx context.Context,
		principal authModels.Principal,
		request models.GetCustomerRequest,
	) (db.Customer, error)

	GetCustomerByName(
		ctx context.Context,
		principal authModels.Principal,
		request models.GetCustomerRequest,
	) ([]db.Customer, error)

	UpdateCustomer(
		ctx context.Context,
		principal authModels.Principal,
		id string,
		request models.UpdateCustomerRequest,
	) error
}
type customerService struct {
	customers repository.CustomerRepository
}

func NewCustomerService(customers repository.CustomerRepository) CustomerService {
	return &customerService{customers: customers}
}

func (c *customerService) CreateCustomer(
	ctx context.Context,
	principal authModels.Principal,
	customerRequest models.CreateCustomerRequest,
) error {
	if !authModels.IsAdmin(principal) {
		return authModels.ErrForbidden
	}

	request := db.CreateCustomerParams{
		FirstName: customerRequest.FirstName,
		LastName:  customerRequest.Lastname,
		Email:     customerRequest.Email,
		Phone: pgtype.Text{
			String: customerRequest.Phone,
			Valid:  customerRequest.Phone != "",
		},
	}

	_, err := c.customers.Create(ctx, request)
	if err != nil {
		return err
	}

	return nil
}
func (c *customerService) GetCustomerByID(
	ctx context.Context,
	principal authModels.Principal,
	getCustomerRequest models.GetCustomerRequest,
) (db.Customer, error) {

	if getCustomerRequest.ID == nil || *getCustomerRequest.ID == "" {
		return db.Customer{}, fmt.Errorf("customer ID is required")
	}

	customerID, err := uuid.Parse(*getCustomerRequest.ID)
	if err != nil {
		return db.Customer{}, fmt.Errorf(
			"invalid customer ID: %w",
			err,
		)
	}

	customer, err := c.customers.GetByID(
		ctx,
		pgtype.UUID{
			Bytes: customerID,
			Valid: true,
		},
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Customer{}, fmt.Errorf("customer not found")
		}

		return db.Customer{}, err
	}
	if !customerOwnedBy(principal, uuid.UUID(customer.ID.Bytes)) {
		return db.Customer{}, fmt.Errorf("customer not found")
	}

	return customer, nil
}

func (c *customerService) GetCustomerByName(
	ctx context.Context,
	principal authModels.Principal,
	getCustomerRequest models.GetCustomerRequest,
) ([]db.Customer, error) {
	if !authModels.IsAdmin(principal) {
		return nil, authModels.ErrForbidden
	}

	var firstName pgtype.Text
	if getCustomerRequest.FirstName != nil &&
		*getCustomerRequest.FirstName != "" {

		firstName = pgtype.Text{
			String: *getCustomerRequest.FirstName,
			Valid:  true,
		}
	}

	var lastName pgtype.Text
	if getCustomerRequest.LastName != nil &&
		*getCustomerRequest.LastName != "" {

		lastName = pgtype.Text{
			String: *getCustomerRequest.LastName,
			Valid:  true,
		}
	}

	queryParams := db.SearchCustomersByNameParams{
		FirstName:   firstName,
		LastName:    lastName,
		LimitCount:  20,
		OffsetCount: 0,
	}

	customers, err := c.customers.SearchByName(ctx, queryParams)
	if err != nil {
		return nil, err
	}

	return customers, nil
}

func (c *customerService) UpdateCustomer(
	ctx context.Context,
	principal authModels.Principal,
	id string,
	updateCustomerRequest models.UpdateCustomerRequest,

) error {

	customerID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid customer ID: %w", err)
	}

	customer, err := c.customers.GetByID(ctx, pgtype.UUID{
		Bytes: customerID,
		Valid: true,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("customer not found")
		}
		return err
	}
	if !customerOwnedBy(principal, uuid.UUID(customer.ID.Bytes)) {
		return fmt.Errorf("customer not found")
	}
	if updateCustomerRequest.Email != nil {
		return authModels.ErrForbidden
	}

	query := db.UpdateCustomerByIDParams{
		ID: pgtype.UUID{
			Bytes: customerID,
			Valid: true,
		},
	}

	if updateCustomerRequest.FirstName != nil {
		query.FirstName = pgtype.Text{
			String: *updateCustomerRequest.FirstName,
			Valid:  true,
		}
	}

	if updateCustomerRequest.LastName != nil {
		query.LastName = pgtype.Text{
			String: *updateCustomerRequest.LastName,
			Valid:  true,
		}
	}

	if updateCustomerRequest.Phone != nil {
		query.Phone = pgtype.Text{
			String: *updateCustomerRequest.Phone,
			Valid:  true,
		}
	}

	_, err = c.customers.UpdateByID(ctx, query)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("customer not found")
		}

		return err
	}

	return nil
}

func customerOwnedBy(principal authModels.Principal, customerID uuid.UUID) bool {
	return authModels.IsAdmin(principal) || customerID == principal.CustomerID
}
