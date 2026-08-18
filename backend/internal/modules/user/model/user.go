package model

type User struct {
	ID       uint64
	Username string
	Email    string
	Role     string
	Roles    []string
}
