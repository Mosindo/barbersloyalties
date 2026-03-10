package customers

import "time"

type Customer struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	FullName   string    `json:"full_name"`
	Phone      string    `json:"phone"`
	Notes      string    `json:"notes"`
	IsArchived bool      `json:"is_archived"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ListParams struct {
	TenantID string
	Search   string
	Limit    int
	Offset   int
}

type CreateInput struct {
	TenantID string
	FullName string
	Phone    string
	Notes    string
}

type UpdateInput struct {
	TenantID string
	ID       string
	FullName *string
	Phone    *string
	Notes    *string
}
