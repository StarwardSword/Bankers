package domain

type Role int

const (
	RoleGuest Role = iota
	RoleUser
	RoleAdmin
)
