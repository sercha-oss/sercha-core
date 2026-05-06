package registrytesting

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
)

// --- TestInMemoryEntityTypeRegistry_Register ---

func TestInMemoryEntityTypeRegistry_Register_Success(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	metadata := pipeline.EntityTypeMetadata{
		ID:             "PERSON",
		DisplayName:    "Person",
		Description:    "A human individual",
		Example:        "John Doe",
		Group:          "PII",
		Source:         "system",
		OwningDetector: "",
	}

	err := registry.Register(context.Background(), metadata)
	if err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}

	// Verify it was registered
	retrieved, found, err := registry.Get(context.Background(), "PERSON")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found {
		t.Fatal("expected entity type to be found")
	}
	if retrieved.ID != "PERSON" {
		t.Errorf("ID = %q, want %q", retrieved.ID, "PERSON")
	}
	if retrieved.DisplayName != "Person" {
		t.Errorf("DisplayName = %q, want %q", retrieved.DisplayName, "Person")
	}
	if retrieved.Description != "A human individual" {
		t.Errorf("Description = %q, want %q", retrieved.Description, "A human individual")
	}
	if retrieved.Example != "John Doe" {
		t.Errorf("Example = %q, want %q", retrieved.Example, "John Doe")
	}
	if retrieved.Group != "PII" {
		t.Errorf("Group = %q, want %q", retrieved.Group, "PII")
	}
	if retrieved.Source != "system" {
		t.Errorf("Source = %q, want %q", retrieved.Source, "system")
	}
	if retrieved.OwningDetector != "" {
		t.Errorf("OwningDetector = %q, want %q", retrieved.OwningDetector, "")
	}
}

func TestInMemoryEntityTypeRegistry_Register_Duplicate_Error(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	metadata := pipeline.EntityTypeMetadata{
		ID:          "EMAIL",
		DisplayName: "Email Address",
		Description: "An email address",
		Example:     "user@example.com",
		Group:       "PII",
		Source:      "system",
	}

	// First registration should succeed
	err := registry.Register(context.Background(), metadata)
	if err != nil {
		t.Fatalf("first Register() error = %v, want nil", err)
	}

	// Second registration with same ID should fail
	err = registry.Register(context.Background(), metadata)
	if err == nil {
		t.Fatal("Register() error = nil, want error for duplicate")
	}

	if !errors.Is(err, ErrDuplicateEntityType) {
		t.Errorf("error should be ErrDuplicateEntityType, got %v", err)
	}
}

// --- TestInMemoryEntityTypeRegistry_Update ---

func TestInMemoryEntityTypeRegistry_Update_Unknown_Error(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	metadata := pipeline.EntityTypeMetadata{
		ID:          "UNKNOWN",
		DisplayName: "Unknown Type",
		Description: "Unknown",
		Example:     "unknown",
		Group:       "",
		Source:      "admin",
	}

	err := registry.Update(context.Background(), metadata)
	if err == nil {
		t.Fatal("Update() error = nil, want error for unknown ID")
	}

	if !errors.Is(err, ErrUnknownEntityType) {
		t.Errorf("error should be ErrUnknownEntityType, got %v", err)
	}
}

func TestInMemoryEntityTypeRegistry_Update_Known_Success(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	original := pipeline.EntityTypeMetadata{
		ID:          "PHONE",
		DisplayName: "Phone Number",
		Description: "Original description",
		Example:     "+1-555-0100",
		Group:       "PII",
		Source:      "system",
	}

	err := registry.Register(context.Background(), original)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Update the metadata
	updated := pipeline.EntityTypeMetadata{
		ID:          "PHONE",
		DisplayName: "Phone Number",
		Description: "Updated description",
		Example:     "+1-555-0200",
		Group:       "Contact",
		Source:      "admin",
	}

	err = registry.Update(context.Background(), updated)
	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	// Verify the update
	retrieved, found, err := registry.Get(context.Background(), "PHONE")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found {
		t.Fatal("expected entity type to be found")
	}

	if retrieved.Description != "Updated description" {
		t.Errorf("Description = %q, want %q", retrieved.Description, "Updated description")
	}
	if retrieved.Example != "+1-555-0200" {
		t.Errorf("Example = %q, want %q", retrieved.Example, "+1-555-0200")
	}
	if retrieved.Group != "Contact" {
		t.Errorf("Group = %q, want %q", retrieved.Group, "Contact")
	}
}

// --- TestInMemoryEntityTypeRegistry_Delete ---

func TestInMemoryEntityTypeRegistry_Delete_Unknown_Error(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	err := registry.Delete(context.Background(), "NONEXISTENT")
	if err == nil {
		t.Fatal("Delete() error = nil, want error for unknown ID")
	}

	if !errors.Is(err, ErrUnknownEntityType) {
		t.Errorf("error should be ErrUnknownEntityType, got %v", err)
	}
}

func TestInMemoryEntityTypeRegistry_Delete_Known_Success(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	metadata := pipeline.EntityTypeMetadata{
		ID:          "TEMP",
		DisplayName: "Temporary",
		Description: "Temporary entity type",
		Example:     "temp",
		Group:       "",
		Source:      "admin",
	}

	// Register first
	err := registry.Register(context.Background(), metadata)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Verify it exists
	_, found, _ := registry.Get(context.Background(), "TEMP")
	if !found {
		t.Fatal("expected entity type to be registered")
	}

	// Delete it
	err = registry.Delete(context.Background(), "TEMP")
	if err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	// Verify it's gone
	_, found, _ = registry.Get(context.Background(), "TEMP")
	if found {
		t.Error("expected entity type to be deleted")
	}
}

// --- TestInMemoryEntityTypeRegistry_Get ---

func TestInMemoryEntityTypeRegistry_Get_Unknown_NotError(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	metadata, found, err := registry.Get(context.Background(), "NONEXISTENT")
	if err != nil {
		t.Errorf("Get() error = %v, want nil (miss is not an error)", err)
	}
	if found {
		t.Error("found = true, want false for unknown ID")
	}

	// Verify zero value is returned
	if metadata.ID != "" {
		t.Errorf("metadata.ID = %q, want %q (zero value)", metadata.ID, "")
	}
	if metadata.DisplayName != "" {
		t.Errorf("metadata.DisplayName = %q, want %q (zero value)", metadata.DisplayName, "")
	}
}

func TestInMemoryEntityTypeRegistry_Get_Known_Success(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	metadata := pipeline.EntityTypeMetadata{
		ID:             "ADDRESS",
		DisplayName:    "Address",
		Description:    "A street address",
		Example:        "123 Main St",
		Group:          "PII",
		Source:         "system",
		OwningDetector: "address-detector",
	}

	if err := registry.Register(context.Background(), metadata); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	retrieved, found, err := registry.Get(context.Background(), "ADDRESS")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}

	if retrieved.OwningDetector != "address-detector" {
		t.Errorf("OwningDetector = %q, want %q", retrieved.OwningDetector, "address-detector")
	}
}

// --- TestInMemoryEntityTypeRegistry_List ---

func TestInMemoryEntityTypeRegistry_List_Empty(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	list, err := registry.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if list == nil {
		t.Error("expected non-nil slice (even if empty)")
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestInMemoryEntityTypeRegistry_List_Multiple(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	types := []pipeline.EntityTypeMetadata{
		{ID: "TYPE1", DisplayName: "Type 1", Source: "system"},
		{ID: "TYPE2", DisplayName: "Type 2", Source: "admin"},
		{ID: "TYPE3", DisplayName: "Type 3", Source: "system"},
	}

	for _, m := range types {
		if err := registry.Register(context.Background(), m); err != nil {
			t.Fatalf("Register(%s) error = %v", m.ID, err)
		}
	}

	list, err := registry.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 items, got %d", len(list))
	}

	// Verify all types are present (order not guaranteed)
	foundIDs := make(map[pipeline.EntityType]bool)
	for _, item := range list {
		foundIDs[item.ID] = true
	}

	if !foundIDs["TYPE1"] || !foundIDs["TYPE2"] || !foundIDs["TYPE3"] {
		t.Error("not all registered types were returned in List")
	}
}

// --- TestInMemoryEntityTypeRegistry_SetOwningDetector ---

func TestInMemoryEntityTypeRegistry_SetOwningDetector_Unknown_Error(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	err := registry.SetOwningDetector(context.Background(), "UNKNOWN", "detector-1")
	if err == nil {
		t.Fatal("SetOwningDetector() error = nil, want error for unknown ID")
	}

	if !errors.Is(err, ErrUnknownEntityType) {
		t.Errorf("error should be ErrUnknownEntityType, got %v", err)
	}
}

func TestInMemoryEntityTypeRegistry_SetOwningDetector_ClaimOwnership(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	metadata := pipeline.EntityTypeMetadata{
		ID:             "PERSON",
		DisplayName:    "Person",
		Description:    "A person",
		Example:        "John Doe",
		Source:         "system",
		OwningDetector: "",
	}

	if err := registry.Register(context.Background(), metadata); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Claim ownership
	err := registry.SetOwningDetector(context.Background(), "PERSON", "detector-1")
	if err != nil {
		t.Fatalf("SetOwningDetector() error = %v, want nil", err)
	}

	// Verify ownership was set
	retrieved, found, _ := registry.Get(context.Background(), "PERSON")
	if !found {
		t.Fatal("entity type should exist")
	}
	if retrieved.OwningDetector != "detector-1" {
		t.Errorf("OwningDetector = %q, want %q", retrieved.OwningDetector, "detector-1")
	}
}

func TestInMemoryEntityTypeRegistry_SetOwningDetector_IdempotentSameOwner(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	metadata := pipeline.EntityTypeMetadata{
		ID:             "EMAIL",
		DisplayName:    "Email",
		Description:    "An email",
		Example:        "user@example.com",
		Source:         "system",
		OwningDetector: "detector-1",
	}

	if err := registry.Register(context.Background(), metadata); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Set same owner again
	err := registry.SetOwningDetector(context.Background(), "EMAIL", "detector-1")
	if err != nil {
		t.Fatalf("SetOwningDetector() error = %v, want nil (idempotent)", err)
	}

	// Verify owner is still set to detector-1
	retrieved, found, _ := registry.Get(context.Background(), "EMAIL")
	if !found {
		t.Fatal("entity type should exist")
	}
	if retrieved.OwningDetector != "detector-1" {
		t.Errorf("OwningDetector = %q, want %q", retrieved.OwningDetector, "detector-1")
	}
}

func TestInMemoryEntityTypeRegistry_SetOwningDetector_ConflictDifferentOwner(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	metadata := pipeline.EntityTypeMetadata{
		ID:             "PHONE",
		DisplayName:    "Phone",
		Description:    "A phone number",
		Example:        "+1-555-0100",
		Source:         "system",
		OwningDetector: "detector-1",
	}

	if err := registry.Register(context.Background(), metadata); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Try to set different owner
	err := registry.SetOwningDetector(context.Background(), "PHONE", "detector-2")
	if err == nil {
		t.Fatal("SetOwningDetector() error = nil, want error for conflicting owner")
	}

	if !errors.Is(err, ErrOwnerConflict) {
		t.Errorf("error should be ErrOwnerConflict, got %v", err)
	}

	// Verify owner was not changed
	retrieved, found, _ := registry.Get(context.Background(), "PHONE")
	if !found {
		t.Fatal("entity type should exist")
	}
	if retrieved.OwningDetector != "detector-1" {
		t.Errorf("OwningDetector = %q, want %q (should be unchanged)", retrieved.OwningDetector, "detector-1")
	}
}

func TestInMemoryEntityTypeRegistry_SetOwningDetector_ClearOwnership(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	metadata := pipeline.EntityTypeMetadata{
		ID:             "ADDRESS",
		DisplayName:    "Address",
		Description:    "An address",
		Example:        "123 Main St",
		Source:         "system",
		OwningDetector: "detector-1",
	}

	if err := registry.Register(context.Background(), metadata); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Clear ownership
	err := registry.SetOwningDetector(context.Background(), "ADDRESS", "")
	if err != nil {
		t.Fatalf("SetOwningDetector() error = %v, want nil", err)
	}

	// Verify ownership was cleared
	retrieved, found, _ := registry.Get(context.Background(), "ADDRESS")
	if !found {
		t.Fatal("entity type should exist")
	}
	if retrieved.OwningDetector != "" {
		t.Errorf("OwningDetector = %q, want %q (empty)", retrieved.OwningDetector, "")
	}
}

func TestInMemoryEntityTypeRegistry_SetOwningDetector_ClearOwnership_IdempotentIfAlreadyUnowned(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	metadata := pipeline.EntityTypeMetadata{
		ID:             "TYPE1",
		DisplayName:    "Type 1",
		Description:    "Type 1",
		Example:        "example",
		Source:         "system",
		OwningDetector: "",
	}

	if err := registry.Register(context.Background(), metadata); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Clear ownership when already unowned
	err := registry.SetOwningDetector(context.Background(), "TYPE1", "")
	if err != nil {
		t.Fatalf("SetOwningDetector() error = %v, want nil (idempotent)", err)
	}

	// Verify still unowned
	retrieved, found, _ := registry.Get(context.Background(), "TYPE1")
	if !found {
		t.Fatal("entity type should exist")
	}
	if retrieved.OwningDetector != "" {
		t.Errorf("OwningDetector = %q, want %q (empty)", retrieved.OwningDetector, "")
	}
}

func TestInMemoryEntityTypeRegistry_SetOwningDetector_SetOwnerWherePreviouslyClaimed(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	metadata := pipeline.EntityTypeMetadata{
		ID:             "TYPE2",
		DisplayName:    "Type 2",
		Description:    "Type 2",
		Example:        "example",
		Source:         "system",
		OwningDetector: "detector-1",
	}

	if err := registry.Register(context.Background(), metadata); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Clear the owner first
	if err := registry.SetOwningDetector(context.Background(), "TYPE2", ""); err != nil {
		t.Fatalf("SetOwningDetector(clear) error = %v", err)
	}

	// Now set a new owner (should succeed after clearing)
	err := registry.SetOwningDetector(context.Background(), "TYPE2", "detector-2")
	if err != nil {
		t.Fatalf("SetOwningDetector() error = %v, want nil", err)
	}

	retrieved, found, _ := registry.Get(context.Background(), "TYPE2")
	if !found {
		t.Fatal("entity type should exist")
	}
	if retrieved.OwningDetector != "detector-2" {
		t.Errorf("OwningDetector = %q, want %q", retrieved.OwningDetector, "detector-2")
	}
}

// --- Concurrency Tests ---

func TestInMemoryEntityTypeRegistry_Concurrent_ReadsAndWrites(t *testing.T) {
	registry := NewInMemoryEntityTypeRegistry()

	// Register a few initial types
	for i := 0; i < 5; i++ {
		id := pipeline.EntityType("TYPE" + string(rune(i+48)))
		metadata := pipeline.EntityTypeMetadata{
			ID:          id,
			DisplayName: "Type " + string(rune(i+48)),
			Description: "Test type",
			Example:     "example",
			Source:      "system",
		}
		if err := registry.Register(context.Background(), metadata); err != nil {
			t.Fatalf("Register(%s) error = %v", id, err)
		}
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 10)
	done := make(chan bool)

	// 5 readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, _, err := registry.Get(context.Background(), pipeline.EntityType("TYPE"+string(rune(idx%5+48))))
				if err != nil {
					errChan <- err
				}
				list, err := registry.List(context.Background())
				if err != nil {
					errChan <- err
				}
				if len(list) < 5 {
					errChan <- errors.New("list returned fewer items than expected")
				}
			}
		}(i)
	}

	// 3 writers (register updates)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				id := pipeline.EntityType("TYPE" + string(rune(idx%5+48)))
				metadata := pipeline.EntityTypeMetadata{
					ID:          id,
					DisplayName: "Type " + string(rune(idx%5+48)),
					Description: "Updated",
					Example:     "example",
					Source:      "admin",
				}
				// Update may race with SetOwningDetector clearing/setting the owner;
				// we don't assert on its error, only on data-race-freedom under -race.
				_ = registry.Update(context.Background(), metadata)
			}
		}(i)
	}

	// 2 writers (set owning detector)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				id := pipeline.EntityType("TYPE" + string(rune(idx%5+48)))
				detectorID := "detector-" + string(rune(idx+48))
				// Concurrent owner sets may conflict; ignore errors here — the
				// test exists to detect data races, not to assert ordering.
				_ = registry.SetOwningDetector(context.Background(), id, detectorID)

				// Clear it again to allow others to set
				_ = registry.SetOwningDetector(context.Background(), id, "")
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for completion and drain any errors collected by readers.
	<-done
	close(errChan)
	for err := range errChan {
		t.Errorf("concurrent operation error: %v", err)
	}
}

func TestInMemoryEntityTypeRegistry_RaceCondition_RegisterParallel(t *testing.T) {
	// This test runs with -race flag to detect race conditions
	registry := NewInMemoryEntityTypeRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := pipeline.EntityType("TYPE" + string(rune(idx+48)))
			metadata := pipeline.EntityTypeMetadata{
				ID:          id,
				DisplayName: "Type " + string(rune(idx+48)),
				Description: "Test",
				Example:     "example",
				Source:      "system",
			}
			// Each goroutine registers a distinct ID; errors here would be a bug.
			if err := registry.Register(context.Background(), metadata); err != nil {
				t.Errorf("Register(%s) error = %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	// Verify all 10 were registered
	list, _ := registry.List(context.Background())
	if len(list) != 10 {
		t.Errorf("expected 10 registered types, got %d", len(list))
	}
}

func TestInMemoryEntityTypeRegistry_RaceCondition_GetAndUpdate(t *testing.T) {
	// This test runs with -race flag to detect race conditions
	registry := NewInMemoryEntityTypeRegistry()

	metadata := pipeline.EntityTypeMetadata{
		ID:          "PERSON",
		DisplayName: "Person",
		Description: "A person",
		Example:     "John Doe",
		Source:      "system",
	}
	if err := registry.Register(context.Background(), metadata); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Concurrent reads — errors are not expected for an existing key,
			// but we don't assert here; the goal is data-race detection.
			_, _, _ = registry.Get(context.Background(), "PERSON")
		}()
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			updated := pipeline.EntityTypeMetadata{
				ID:          "PERSON",
				DisplayName: "Person",
				Description: "Updated " + string(rune(idx+48)),
				Example:     "Jane Doe",
				Source:      "admin",
			}
			// Concurrent writes — errors not asserted; race detection is the point.
			_ = registry.Update(context.Background(), updated)
		}(i)
	}
	wg.Wait()
}
