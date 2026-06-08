package portal

import (
	"context"
	"errors"
	"testing"
	"time"

	cpadashboard "github.com/router-for-me/CLIProxyAPI/v7/internal/cpa_dashboard"
)

type fakePortalDashboardStore struct{}

func (f *fakePortalDashboardStore) QuotaObservationWindows(context.Context, cpadashboard.QueryFilter) ([]cpadashboard.QuotaObservationWindow, error) {
	return nil, nil
}

func (f *fakePortalDashboardStore) QuotaPoints(context.Context, cpadashboard.QueryFilter) ([]cpadashboard.QuotaPoint, error) {
	return nil, nil
}

func (f *fakePortalDashboardStore) UsageSummaries(context.Context, cpadashboard.QueryFilter) ([]cpadashboard.UsageSummary, error) {
	return nil, nil
}

func (f *fakePortalDashboardStore) LatestCollectionAt(context.Context) (*time.Time, error) {
	return nil, nil
}

func (f *fakePortalDashboardStore) CurrentAccounts(context.Context, string) ([]cpadashboard.CPAAccountRow, error) {
	return nil, nil
}

func (f *fakePortalDashboardStore) UsageBuckets(context.Context, cpadashboard.QueryFilter) ([]cpadashboard.UsageBucket, error) {
	return nil, nil
}

func (f *fakePortalDashboardStore) CleanupCandidates(context.Context, string) ([]cpadashboard.CleanupCandidate, error) {
	return nil, nil
}

func (f *fakePortalDashboardStore) Filters(context.Context) (cpadashboard.FiltersResponse, error) {
	return cpadashboard.FiltersResponse{
		CPASources: []cpadashboard.FilterOption{{ID: "cpa1", Label: "cpa1"}},
	}, nil
}

func TestAdminDashboardRequiresAdminAndConfiguredService(t *testing.T) {
	service := NewService(Config{}, nil, nil)
	admin := User{ID: 1, Role: roleAdmin}
	user := User{ID: 2, Role: roleUser}

	if _, errFilters := service.AdminDashboardFilters(t.Context(), admin); !errors.Is(errFilters, ErrNotFound) {
		t.Fatalf("AdminDashboardFilters without dashboard error = %v, want ErrNotFound", errFilters)
	}

	service.SetDashboardService(cpadashboard.NewService(&fakePortalDashboardStore{}))
	if _, errFilters := service.AdminDashboardFilters(t.Context(), user); !errors.Is(errFilters, ErrForbidden) {
		t.Fatalf("AdminDashboardFilters as user error = %v, want ErrForbidden", errFilters)
	}

	filters, errFilters := service.AdminDashboardFilters(t.Context(), admin)
	if errFilters != nil {
		t.Fatalf("AdminDashboardFilters as admin returned error: %v", errFilters)
	}
	if len(filters.CPASources) != 1 || filters.CPASources[0].ID != "cpa1" {
		t.Fatalf("filters = %#v, want cpa1 source", filters)
	}
}
