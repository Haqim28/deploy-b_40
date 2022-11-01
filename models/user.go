package models

import "time"

type User struct {
	ID        int                   `json:"id"`
	Name      string                `json:"name" gorm:"type: varchar(255)"`
	Email     string                `json:"email" gorm:"type: varchar(255)"`
	Password  string                `json:"-" gorm:"type: varchar(255)"`
	Status    string                `json:"status" gorm:"type: varchar(50)"`
	Profile   ProfileResponse       `json:"profile"`
	Products  []ProductUserResponse `json:"products" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	CreatedAt time.Time             `json:"-"`
	UpdatedAt time.Time             `json:"-"`
}

type UsersProfileResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Email string `json:"email"`
}

func (UsersProfileResponse) TableName() string {
	return "users"
}
