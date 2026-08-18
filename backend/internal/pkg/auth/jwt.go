package auth

type Claims struct {
	UserID   uint64   `json:"user_id"`
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Roles    []string `json:"roles"`
}

func BuildMockClaims() *Claims {
	return &Claims{
		UserID:   1,
		Username: "admin",
		Role:     "admin",
		Roles:    []string{"admin"},
	}
}
