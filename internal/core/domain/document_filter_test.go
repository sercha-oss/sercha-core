package domain_test

import (
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

func TestDocumentIDFilter_IsDenyAll(t *testing.T) {
	tests := []struct {
		name   string
		filter *domain.DocumentIDFilter
		want   bool
	}{
		{"nil receiver", nil, false},
		{"Apply=false is not deny-all", &domain.DocumentIDFilter{Apply: false, IDs: nil}, false},
		{"Apply=false with IDs is not deny-all", &domain.DocumentIDFilter{Apply: false, IDs: []string{"a"}}, false},
		{"Apply=true with nil IDs is deny-all", &domain.DocumentIDFilter{Apply: true, IDs: nil}, true},
		{"Apply=true with empty IDs is deny-all", &domain.DocumentIDFilter{Apply: true, IDs: []string{}}, true},
		{"Apply=true with non-empty IDs is not deny-all", &domain.DocumentIDFilter{Apply: true, IDs: []string{"a"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.IsDenyAll(); got != tt.want {
				t.Errorf("IsDenyAll() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDocumentIDFilter_IsAllowList(t *testing.T) {
	tests := []struct {
		name   string
		filter *domain.DocumentIDFilter
		want   bool
	}{
		{"nil receiver", nil, false},
		{"Apply=false with IDs is not allow-list", &domain.DocumentIDFilter{Apply: false, IDs: []string{"a"}}, false},
		{"Apply=true with empty IDs is not allow-list", &domain.DocumentIDFilter{Apply: true, IDs: []string{}}, false},
		{"Apply=true with nil IDs is not allow-list", &domain.DocumentIDFilter{Apply: true, IDs: nil}, false},
		{"Apply=true with one ID is allow-list", &domain.DocumentIDFilter{Apply: true, IDs: []string{"a"}}, true},
		{"Apply=true with many IDs is allow-list", &domain.DocumentIDFilter{Apply: true, IDs: []string{"a", "b", "c"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.IsAllowList(); got != tt.want {
				t.Errorf("IsAllowList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDenyAllDocumentIDFilter(t *testing.T) {
	f := domain.DenyAllDocumentIDFilter()
	if f == nil {
		t.Fatal("DenyAllDocumentIDFilter() returned nil")
	}
	if !f.Apply {
		t.Error("Apply = false, want true")
	}
	if len(f.IDs) != 0 {
		t.Errorf("IDs = %v, want empty", f.IDs)
	}
	if !f.IsDenyAll() {
		t.Error("IsDenyAll() = false, want true")
	}
	if f.IsAllowList() {
		t.Error("IsAllowList() = true, want false")
	}
}

func TestAllowDocumentIDs(t *testing.T) {
	t.Run("non-empty ids produces allow-list", func(t *testing.T) {
		f := domain.AllowDocumentIDs([]string{"a", "b"})
		if !f.IsAllowList() {
			t.Error("IsAllowList() = false, want true")
		}
		if f.IsDenyAll() {
			t.Error("IsDenyAll() = true, want false")
		}
		if len(f.IDs) != 2 || f.IDs[0] != "a" || f.IDs[1] != "b" {
			t.Errorf("IDs = %v, want [a b]", f.IDs)
		}
	})

	t.Run("nil ids normalises to deny-all", func(t *testing.T) {
		f := domain.AllowDocumentIDs(nil)
		if !f.IsDenyAll() {
			t.Error("nil input: IsDenyAll() = false, want true (empty allow-list = deny all)")
		}
		if f.IDs == nil {
			t.Error("IDs = nil, want empty non-nil slice")
		}
	})

	t.Run("empty ids produces deny-all", func(t *testing.T) {
		f := domain.AllowDocumentIDs([]string{})
		if !f.IsDenyAll() {
			t.Error("empty input: IsDenyAll() = false, want true (empty allow-list = deny all)")
		}
	})
}
