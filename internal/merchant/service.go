package merchant

import "context"

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo} }

func (s *Service) Register(ctx context.Context, name, email string, webhook *string) (*Merchant, error) {
	m := &Merchant{Name: name, Email: email, WebhookURL: webhook}
	if webhook != nil && *webhook == "" {
		m.WebhookURL = nil
	}
	return m, s.repo.Create(ctx, m)
}
