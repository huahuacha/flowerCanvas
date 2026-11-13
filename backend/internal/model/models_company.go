package model

import "time"

type CompanyMemberRole string

const (
	CompanyMemberRoleAdmin    CompanyMemberRole = "admin"
	CompanyMemberRoleEmployee CompanyMemberRole = "employee"
)

type Company struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	Name      string    `json:"name" gorm:"size:120;not null"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CompanyMember records company authority separately from the user's global role.
type CompanyMember struct {
	ID        string            `json:"id" gorm:"primaryKey;size:36"`
	CompanyID string            `json:"companyId" gorm:"size:36;not null;index:idx_company_members_company"`
	UserID    string            `json:"userId" gorm:"size:36;not null;uniqueIndex:idx_company_members_user"`
	Role      CompanyMemberRole `json:"role" gorm:"size:24;not null;index"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}
