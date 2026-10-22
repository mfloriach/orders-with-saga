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

func (s *Service) List(ctx context.Context, limit, offset int) ([]domain.Merchant, int, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Merchant, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id string, name, email, webhook *string) (*domain.Merchant, error) {
	if name == nil && email == nil && webhook == nil {
		return s.repo.FindByID(ctx, id)
	}
	return s.repo.Update(ctx, id, name, email, webhook)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
