package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/sathishkumar-nce/amz-orders/internal/integrations/interakt"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/repository"
)

type InteraktSettingsService struct {
	repo     *repository.InteraktSettingsRepository
	client   *interakt.Client
	defaults models.InteraktSettings
}

func NewInteraktSettingsService(repo *repository.InteraktSettingsRepository, client *interakt.Client, defaults models.InteraktSettings) *InteraktSettingsService {
	return &InteraktSettingsService{
		repo:     repo,
		client:   client,
		defaults: defaults,
	}
}

func (s *InteraktSettingsService) Get(ctx context.Context) (*models.InteraktSettings, error) {
	settings, err := s.repo.Get(ctx, s.defaults)
	if err != nil {
		return nil, err
	}
	s.applyRuntime(settings)
	return settings, nil
}

func (s *InteraktSettingsService) Update(ctx context.Context, settings models.InteraktSettings) (*models.InteraktSettings, error) {
	settings.Mode = normalizeInteraktMode(settings.Mode)
	if settings.Mode == "" {
		return nil, fmt.Errorf("mode must be either test or prod")
	}
	settings.TemplateName = strings.TrimSpace(settings.TemplateName)
	if settings.TemplateName == "" {
		return nil, fmt.Errorf("template_name is required")
	}
	settings.TestNumber = strings.TrimSpace(settings.TestNumber)
	if settings.Mode == "test" && settings.TestNumber == "" {
		return nil, fmt.Errorf("test_number is required when mode is test")
	}
	if err := s.repo.Update(ctx, settings); err != nil {
		return nil, err
	}
	s.applyRuntime(&settings)
	return &settings, nil
}

func normalizeInteraktMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "test":
		return "test"
	case "prod":
		return "prod"
	default:
		return ""
	}
}

func (s *InteraktSettingsService) applyRuntime(settings *models.InteraktSettings) {
	if s.client == nil || settings == nil {
		return
	}
	s.client.UpdateRuntimeSettings(settings.Enabled, settings.Mode, settings.TemplateName, settings.TestNumber)
}
