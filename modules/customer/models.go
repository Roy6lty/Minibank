package customer

type CreateCustomerRequest struct {
	FirstName string `json:"first_name"`
	Lastname  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type UpdateCustomerRequest struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Email     *string `json:"email"`
	Phone     *string `json:"phone"`
}

type GetCustomerRequest struct {
	ID            *string `json:"userID"`
	FirstName     *string `json:"first_name"`
	LastName      *string `json:"last_name"`
	Email         *string `json:"email"`
	Phone         *string `json:"phone"`
	AccountNumber *string `json:" accountNumber"`
}
