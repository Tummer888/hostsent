package dto

import "time"

type UserPermissionItem struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

type UserInstanceItem struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Region    string    `json:"region"`
	Specs     string    `json:"specs"`
	Status    string    `json:"status"`
	ExpireAt  time.Time `json:"expire_at"`
}

type UserOrderItem struct {
	ID        uint64    `json:"id"`
	OrderNo   string    `json:"order_no"`
	Product   string    `json:"product"`
	Amount    float64   `json:"amount"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type UserBillItem struct {
	ID           uint64  `json:"id"`
	BillingMonth string  `json:"billing_month"`
	Amount       float64 `json:"amount"`
	Status       string  `json:"status"`
}

type UserTransactionItem struct {
	ID        uint64    `json:"id"`
	TxnNo     string    `json:"txn_no"`
	Type      string    `json:"type"`
	Amount    float64   `json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}

type UserTicketItem struct {
	ID        uint64    `json:"id"`
	TicketNo  string    `json:"ticket_no"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	Priority  string    `json:"priority"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserDetailAggregateResponse struct {
	Profile      UserInfo                `json:"profile"`
	Permissions  []UserPermissionItem    `json:"permissions"`
	Instances    []UserInstanceItem      `json:"instances"`
	Orders       []UserOrderItem         `json:"orders"`
	Bills        []UserBillItem          `json:"bills"`
	Transactions []UserTransactionItem   `json:"transactions"`
	Tickets      []UserTicketItem        `json:"tickets"`
}
