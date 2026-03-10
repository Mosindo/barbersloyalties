package tenants

import "time"

type Tenant struct {
	ID           string    `json:"id"`
	BusinessName string    `json:"business_name"`
	OwnerName    string    `json:"owner_name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateTenantInput struct {
	BusinessName string
	OwnerName    string
	Email        string
	Phone        string
}
