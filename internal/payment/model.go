package payment

import "time"

type Payment struct {
	ID            string    `json:"id"`
	MerchantID    string    `json:"merchant_id"`
	Amount        int64     `json:"amount"`
	IdempotencyKey string   `json:"idempotency_key"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
