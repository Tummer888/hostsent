package model

import "time"

type UserInstance struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	UserID     uint64    `gorm:"column:user_id;index;not null"`
	Name       string    `gorm:"size:128;not null"`
	Region     string    `gorm:"size:64;not null"`
	Specs      string    `gorm:"size:255;not null"`
	Status     string    `gorm:"size:32;not null;default:active"`
	ExpireAt   time.Time `gorm:"column:expire_at;not null"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"`
}

func (UserInstance) TableName() string {
	return "user_instances"
}

type UserOrder struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	UserID     uint64    `gorm:"column:user_id;index;not null"`
	OrderNo    string    `gorm:"column:order_no;size:64;uniqueIndex;not null"`
	Product    string    `gorm:"size:128;not null"`
	Amount     float64   `gorm:"type:decimal(15,2);not null;default:0"`
	Status     string    `gorm:"size:32;not null;default:pending"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"`
}

func (UserOrder) TableName() string {
	return "user_orders"
}

type UserBill struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	UserID      uint64    `gorm:"column:user_id;index;not null"`
	BillingMonth string   `gorm:"column:billing_month;size:16;not null"`
	Amount      float64   `gorm:"type:decimal(15,2);not null;default:0"`
	Status      string    `gorm:"size:32;not null;default:pending"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (UserBill) TableName() string {
	return "user_bills"
}

type UserTransaction struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	UserID      uint64    `gorm:"column:user_id;index;not null"`
	TxnNo       string    `gorm:"column:txn_no;size:64;uniqueIndex;not null"`
	Type        string    `gorm:"size:32;not null"`
	Amount      float64   `gorm:"type:decimal(15,2);not null;default:0"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (UserTransaction) TableName() string {
	return "user_transactions"
}

type UserTicket struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	UserID     uint64    `gorm:"column:user_id;index;not null"`
	TicketNo   string    `gorm:"column:ticket_no;size:64;uniqueIndex;not null"`
	Title      string    `gorm:"size:255;not null"`
	Category   string    `gorm:"size:64;not null"`
	Priority   string    `gorm:"size:32;not null;default:medium"`
	Status     string    `gorm:"size:32;not null;default:open"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

func (UserTicket) TableName() string {
	return "user_tickets"
}
