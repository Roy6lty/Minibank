package customer

type CreateCustomerRequest struct {
	FirstName string `json:"first_name" validate:"required,max=100"`
	Lastname  string `json:"last_name" validate:"required,max=100"`
	Email     string `json:"email" validate:"required,email,max=254"`
	Phone     string `json:"phone" validate:"omitempty,max=32"`
}

type UpdateCustomerRequest struct {
	FirstName *string `json:"first_name" validate:"omitempty,max=100"`
	LastName  *string `json:"last_name" validate:"omitempty,max=100"`
	Email     *string `json:"email" validate:"omitempty,email,max=254"`
	Phone     *string `json:"phone" validate:"omitempty,max=32"`
}

type GetCustomerRequest struct {
	ID            *string `json:"userID"`
	FirstName     *string `json:"first_name"`
	LastName      *string `json:"last_name"`
	Email         *string `json:"email"`
	Phone         *string `json:"phone"`
	AccountNumber *string `json:" accountNumber"`
}
