package merchant

import (
	"context"
	"paymentgateway/internal/domain"
)

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo} }

func (s *Service) Register(ctx context.Context, name, email string, webhook *string) (*domain.Merchant, error) {
	m := &domain.Merchant{Name: name, Email: email, WebhookURL: webhook}
	if webhook != nil && *webhook == "" {
		m.WebhookURL = nil
	}
	return m, s.repo.Create(ctx, m)
}
