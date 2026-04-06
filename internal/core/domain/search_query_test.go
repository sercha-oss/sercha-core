package domain

import (
	"testing"
	"time"
)

func TestNewSearchQuery(t *testing.T) {
	teamID := "team-123"
	userID := "user-456"
	query := "test search query"
	mode := SearchModeHybrid
	resultCount := 42
	duration := 150 * time.Millisecond

	sq := NewSearchQuery(teamID, userID, query, mode, resultCount, duration)

	if sq.ID == "" {
		t.Error("expected non-empty ID")
	}
	if sq.TeamID != teamID {
		t.Errorf("expected team ID %s, got %s", teamID, sq.TeamID)
	}
	if sq.UserID != userID {
		t.Errorf("expected user ID %s, got %s", userID, sq.UserID)
	}
	if sq.Query != query {
		t.Errorf("expected query %s, got %s", query, sq.Query)
	}
	if sq.Mode != mode {
		t.Errorf("expected mode %s, got %s", mode, sq.Mode)
	}
	if sq.ResultCount != resultCount {
		t.Errorf("expected result count %d, got %d", resultCount, sq.ResultCount)
	}
	if sq.Duration != duration.Nanoseconds() {
		t.Errorf("expected duration %d nanoseconds, got %d", duration.Nanoseconds(), sq.Duration)
	}
	if sq.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestSearchQuery_GetDuration(t *testing.T) {
	sq := NewSearchQuery("team", "user", "query", SearchModeHybrid, 10, 250*time.Millisecond)

	duration := sq.GetDuration()
	expected := 250 * time.Millisecond

	if duration != expected {
		t.Errorf("expected duration %v, got %v", expected, duration)
	}
}

func TestSearchQuery_WithSourceFilters(t *testing.T) {
	sq := NewSearchQuery("team", "user", "query", SearchModeHybrid, 10, 100*time.Millisecond)

	// Initially no filters
	if sq.HasFilters {
		t.Error("expected HasFilters to be false initially")
	}
	if len(sq.SourceIDs) != 0 {
		t.Error("expected empty SourceIDs initially")
	}

	// Add source filters
	sourceIDs := []string{"src-1", "src-2"}
	sq.WithSourceFilters(sourceIDs)

	if !sq.HasFilters {
		t.Error("expected HasFilters to be true after adding filters")
	}
	if len(sq.SourceIDs) != 2 {
		t.Errorf("expected 2 source IDs, got %d", len(sq.SourceIDs))
	}
	if sq.SourceIDs[0] != "src-1" || sq.SourceIDs[1] != "src-2" {
		t.Error("source IDs not set correctly")
	}
}

func TestSearchQuery_WithFilters(t *testing.T) {
	sq := NewSearchQuery("team", "user", "query", SearchModeHybrid, 10, 100*time.Millisecond)

	// Initially no filters
	if sq.HasFilters {
		t.Error("expected HasFilters to be false initially")
	}

	// Mark as having filters
	sq.WithFilters(true)

	if !sq.HasFilters {
		t.Error("expected HasFilters to be true after WithFilters(true)")
	}
}

func TestNewAnalyticsPeriod(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	period := NewAnalyticsPeriod(start, end)

	if !period.Start.Equal(start) {
		t.Errorf("expected start %v, got %v", start, period.Start)
	}
	if !period.End.Equal(end) {
		t.Errorf("expected end %v, got %v", end, period.End)
	}
}

func TestLast24Hours(t *testing.T) {
	before := time.Now()
	period := Last24Hours()
	after := time.Now()

	// Check that the period is approximately 24 hours
	duration := period.End.Sub(period.Start)
	expectedDuration := 24 * time.Hour

	if duration < expectedDuration-time.Second || duration > expectedDuration+time.Second {
		t.Errorf("expected duration around %v, got %v", expectedDuration, duration)
	}

	// Check that end time is close to now
	if period.End.Before(before) || period.End.After(after.Add(time.Second)) {
		t.Error("expected end time to be close to now")
	}
}

func TestLast7Days(t *testing.T) {
	period := Last7Days()
	duration := period.End.Sub(period.Start)
	expectedDuration := 7 * 24 * time.Hour

	if duration < expectedDuration-time.Second || duration > expectedDuration+time.Second {
		t.Errorf("expected duration around %v, got %v", expectedDuration, duration)
	}
}

func TestLast30Days(t *testing.T) {
	period := Last30Days()
	duration := period.End.Sub(period.Start)
	expectedDuration := 30 * 24 * time.Hour

	if duration < expectedDuration-time.Second || duration > expectedDuration+time.Second {
		t.Errorf("expected duration around %v, got %v", expectedDuration, duration)
	}
}

func TestSearchAnalytics_Structure(t *testing.T) {
	analytics := &SearchAnalytics{
		TotalSearches:   1000,
		UniqueUsers:     50,
		AverageDuration: 125.5,
		AverageResults:  15.3,
		TopQueries: []QueryFrequency{
			{Query: "test", Count: 100},
			{Query: "search", Count: 50},
		},
		SearchesByMode: map[SearchMode]int64{
			SearchModeHybrid:       600,
			SearchModeTextOnly:     300,
			SearchModeSemanticOnly: 100,
		},
		Period: Last24Hours(),
	}

	if analytics.TotalSearches != 1000 {
		t.Error("TotalSearches not set correctly")
	}
	if analytics.UniqueUsers != 50 {
		t.Error("UniqueUsers not set correctly")
	}
	if len(analytics.TopQueries) != 2 {
		t.Error("TopQueries not set correctly")
	}
	if len(analytics.SearchesByMode) != 3 {
		t.Error("SearchesByMode not set correctly")
	}
}

func TestSearchMetrics_Structure(t *testing.T) {
	metrics := &SearchMetrics{
		FastSearches:       800,
		MediumSearches:     150,
		SlowSearches:       50,
		P50Duration:        85.5,
		P95Duration:        450.0,
		P99Duration:        800.0,
		ZeroResultSearches: 20,
		Period:             Last24Hours(),
	}

	if metrics.FastSearches != 800 {
		t.Error("FastSearches not set correctly")
	}
	if metrics.MediumSearches != 150 {
		t.Error("MediumSearches not set correctly")
	}
	if metrics.SlowSearches != 50 {
		t.Error("SlowSearches not set correctly")
	}
	if metrics.ZeroResultSearches != 20 {
		t.Error("ZeroResultSearches not set correctly")
	}
}
