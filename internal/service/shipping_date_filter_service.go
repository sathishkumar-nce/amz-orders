package service

import (
	"context"
	"fmt"

	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/repository"
)

type ShippingDateFilterService struct {
	repo *repository.ShippingDateFilterRepository
}

func NewShippingDateFilterService(repo *repository.ShippingDateFilterRepository) *ShippingDateFilterService {
	return &ShippingDateFilterService{repo: repo}
}

func (s *ShippingDateFilterService) List(ctx context.Context) ([]models.ShippingDateFilterSetting, error) {
	return s.repo.List(ctx)
}

func (s *ShippingDateFilterService) GetActiveFilterKey(ctx context.Context) (string, error) {
	return s.repo.GetActiveFilterKey(ctx)
}

func (s *ShippingDateFilterService) UpdateAll(ctx context.Context, filters []models.ShippingDateFilterSetting) error {
	allowed := map[string]struct{}{
		"prime_ship_today":             {},
		"prime_ship_tomorrow":          {},
		"one_day_handle_ship_today":    {},
		"one_day_handle_ship_tomorrow": {},
		"custom":                       {},
		"all_date":                     {},
	}

	for index := range filters {
		filter := &filters[index]
		if _, ok := allowed[filter.FilterKey]; !ok {
			return fmt.Errorf("unsupported filter_key: %s", filter.FilterKey)
		}
		if filter.Label == "" {
			return fmt.Errorf("label is required for %s", filter.FilterKey)
		}
		if filter.FilterKey == "all_date" {
			filter.IsRangeEnabled = false
			continue
		}
		if filter.StartHour < 0 || filter.StartHour > 23 || filter.EndHour < 0 || filter.EndHour > 23 {
			return fmt.Errorf("hours must be between 0 and 23 for %s", filter.FilterKey)
		}
		if filter.StartMinute < 0 || filter.StartMinute > 59 || filter.EndMinute < 0 || filter.EndMinute > 59 {
			return fmt.Errorf("minutes must be between 0 and 59 for %s", filter.FilterKey)
		}
		filter.IsRangeEnabled = true
	}

	return s.repo.UpdateAll(ctx, filters)
}

func (s *ShippingDateFilterService) ResetDefaults(ctx context.Context) error {
	return s.repo.ResetDefaults(ctx)
}

func (s *ShippingDateFilterService) SetActiveFilterKey(ctx context.Context, filterKey string) error {
	allowed := map[string]struct{}{
		"prime_ship_today":             {},
		"prime_ship_tomorrow":          {},
		"one_day_handle_ship_today":    {},
		"one_day_handle_ship_tomorrow": {},
		"custom":                       {},
		"all_date":                     {},
	}

	if _, ok := allowed[filterKey]; !ok {
		return fmt.Errorf("unsupported filter_key: %s", filterKey)
	}

	return s.repo.SetActiveFilterKey(ctx, filterKey)
}
