package search

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

// --- Test fakes for EntityDetector and EntityRegister ---

type fakeDetector struct {
	spans    []pipeline.EntitySpan
	err      error
	callCount int
	lastText string
}

func (f *fakeDetector) Detect(_ context.Context, text string) ([]pipeline.EntitySpan, error) {
	f.callCount++
	f.lastText = text
	return f.spans, f.err
}

func (f *fakeDetector) OwnedCategories() []pipeline.EntityType {
	return []pipeline.EntityType{"PERSON", "EMAIL"}
}

type fakeRegister struct {
	getCalls   []fakeRegisterGetCall
	putCalls   []driven.EntityAnalysis
	getErr     error
	putErr     error
	getHit     bool
	getAnalysis *driven.EntityAnalysis
}

type fakeRegisterGetCall struct {
	docID           string
	contentSHA256   string
	analyzerVersion string
}

func (f *fakeRegister) Get(_ context.Context, docID, contentSHA256, analyzerVersion string) (*driven.EntityAnalysis, bool, error) {
	f.getCalls = append(f.getCalls, fakeRegisterGetCall{docID, contentSHA256, analyzerVersion})
	if f.getErr != nil {
		return nil, false, f.getErr
	}
	if f.getHit && f.getAnalysis != nil {
		return f.getAnalysis, true, nil
	}
	return nil, false, nil
}

func (f *fakeRegister) Put(_ context.Context, analysis *driven.EntityAnalysis) error {
	f.putCalls = append(f.putCalls, *analysis)
	return f.putErr
}

type fakeRegistry struct {
	types map[pipeline.EntityType]pipeline.EntityTypeMetadata
}

func (f *fakeRegistry) Register(_ context.Context, metadata pipeline.EntityTypeMetadata) error {
	f.types[metadata.ID] = metadata
	return nil
}

func (f *fakeRegistry) Update(_ context.Context, metadata pipeline.EntityTypeMetadata) error {
	f.types[metadata.ID] = metadata
	return nil
}

func (f *fakeRegistry) Delete(_ context.Context, id pipeline.EntityType) error {
	delete(f.types, id)
	return nil
}

func (f *fakeRegistry) Get(_ context.Context, id pipeline.EntityType) (pipeline.EntityTypeMetadata, bool, error) {
	m, found := f.types[id]
	return m, found, nil
}

func (f *fakeRegistry) List(_ context.Context) ([]pipeline.EntityTypeMetadata, error) {
	out := make([]pipeline.EntityTypeMetadata, 0, len(f.types))
	for _, m := range f.types {
		out = append(out, m)
	}
	return out, nil
}

func (f *fakeRegistry) SetOwningDetector(_ context.Context, id pipeline.EntityType, detectorID string) error {
	m, found := f.types[id]
	if !found {
		return errors.New("not found")
	}
	m.OwningDetector = detectorID
	f.types[id] = m
	return nil
}

// --- EntityExtractorFactory Tests ---

func TestEntityExtractorFactory_StageID(t *testing.T) {
	factory := NewEntityExtractorFactory()
	if factory.StageID() != EntityExtractorStageID {
		t.Errorf("StageID() = %q, want %q", factory.StageID(), EntityExtractorStageID)
	}
}

func TestEntityExtractorFactory_Descriptor(t *testing.T) {
	factory := NewEntityExtractorFactory()
	desc := factory.Descriptor()

	if desc.ID != EntityExtractorStageID {
		t.Errorf("ID = %q, want %q", desc.ID, EntityExtractorStageID)
	}
	if desc.Name != "Entity Extractor" {
		t.Errorf("Name = %q, want %q", desc.Name, "Entity Extractor")
	}
	if desc.Type != pipeline.StageTypeEnricher {
		t.Errorf("Type = %q, want %q", desc.Type, pipeline.StageTypeEnricher)
	}
	if desc.InputShape != pipeline.ShapeCandidate {
		t.Errorf("InputShape = %q, want %q", desc.InputShape, pipeline.ShapeCandidate)
	}
	if desc.OutputShape != pipeline.ShapeCandidate {
		t.Errorf("OutputShape = %q, want %q", desc.OutputShape, pipeline.ShapeCandidate)
	}
	if desc.Cardinality != pipeline.CardinalityOneToOne {
		t.Errorf("Cardinality = %q, want %q", desc.Cardinality, pipeline.CardinalityOneToOne)
	}
	if desc.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", desc.Version, "1.0.0")
	}

	// Verify capabilities
	if len(desc.Capabilities) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(desc.Capabilities))
	}

	capTypes := make(map[pipeline.CapabilityType]bool)
	for _, cap := range desc.Capabilities {
		if cap.Mode != pipeline.CapabilityOptional {
			t.Errorf("capability %q mode = %q, want %q", cap.Type, cap.Mode, pipeline.CapabilityOptional)
		}
		capTypes[cap.Type] = true
	}

	if !capTypes[pipeline.CapabilityEntityDetector] {
		t.Error("missing CapabilityEntityDetector")
	}
	if !capTypes[pipeline.CapabilityEntityRegister] {
		t.Error("missing CapabilityEntityRegister")
	}
	if !capTypes[pipeline.CapabilityEntityTypeRegistry] {
		t.Error("missing CapabilityEntityTypeRegistry")
	}
}

func TestEntityExtractorFactory_Validate(t *testing.T) {
	factory := NewEntityExtractorFactory()
	config := pipeline.StageConfig{
		StageID: EntityExtractorStageID,
		Enabled: true,
	}

	if err := factory.Validate(config); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestEntityExtractorFactory_Create_WithAllCapabilities(t *testing.T) {
	factory := NewEntityExtractorFactory()
	detector := &fakeDetector{}
	register := &fakeRegister{}
	registry := &fakeRegistry{types: make(map[pipeline.EntityType]pipeline.EntityTypeMetadata)}

	capSet := pipeline.NewCapabilitySet()
	capSet.Add(pipeline.CapabilityEntityDetector, "test-detector", detector)
	capSet.Add(pipeline.CapabilityEntityRegister, "test-register", register)
	capSet.Add(pipeline.CapabilityEntityTypeRegistry, "test-registry", registry)

	config := pipeline.StageConfig{
		StageID: EntityExtractorStageID,
		Enabled: true,
	}

	stage, err := factory.Create(config, capSet)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if stage == nil {
		t.Fatal("Create() returned nil stage")
	}

	extStage, ok := stage.(*EntityExtractorStage)
	if !ok {
		t.Fatalf("stage should be *EntityExtractorStage, got %T", stage)
	}
	if extStage.detector == nil {
		t.Error("detector should be set")
	}
	if extStage.register == nil {
		t.Error("register should be set")
	}
	if extStage.registry == nil {
		t.Error("registry should be set")
	}
}

func TestEntityExtractorFactory_Create_WithoutCapabilities(t *testing.T) {
	factory := NewEntityExtractorFactory()
	capSet := pipeline.NewCapabilitySet()

	config := pipeline.StageConfig{
		StageID: EntityExtractorStageID,
		Enabled: true,
	}

	stage, err := factory.Create(config, capSet)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil (capabilities are optional)", err)
	}
	if stage == nil {
		t.Fatal("Create() returned nil stage")
	}

	extStage, ok := stage.(*EntityExtractorStage)
	if !ok {
		t.Fatalf("stage should be *EntityExtractorStage, got %T", stage)
	}
	if extStage.detector != nil {
		t.Error("detector should be nil when not available")
	}
	if extStage.register != nil {
		t.Error("register should be nil when not available")
	}
	if extStage.registry != nil {
		t.Error("registry should be nil when not available")
	}
}

// --- EntityExtractorStage Process Tests ---

func TestEntityExtractorStage_Process_NilDetector_PassThrough(t *testing.T) {
	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   nil,
		register:   nil,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{
		{
			DocumentID: "doc1",
			Content:    "John Doe lives in New York",
			Metadata: map[string]any{
				"existing_key": "existing_value",
			},
		},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v, want nil", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	if len(outCandidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(outCandidates))
	}

	// Metadata should not be modified
	if _, has := outCandidates[0].Metadata["entities"]; has {
		t.Error("entities key should not be present when detector is nil")
	}
	if _, has := outCandidates[0].Metadata["entity_summary"]; has {
		t.Error("entity_summary key should not be present when detector is nil")
	}

	// Original metadata should be preserved
	if outCandidates[0].Metadata["existing_key"] != "existing_value" {
		t.Error("existing metadata should be preserved")
	}
}

func TestEntityExtractorStage_Process_WrongInputType(t *testing.T) {
	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   &fakeDetector{},
		logger:     slog.Default(),
	}

	invalidInputs := []any{
		"string input",
		123,
		[]string{"slice"},
		map[string]string{"key": "value"},
		nil,
	}

	for _, input := range invalidInputs {
		result, err := stage.Process(context.Background(), input)
		if err == nil {
			t.Fatal("Process() error = nil, want error for invalid input")
		}
		if result != nil {
			t.Error("result should be nil on error")
		}

		stageErr, ok := err.(*StageError)
		if !ok {
			t.Fatalf("error should be *StageError, got %T", err)
		}
		if stageErr.Stage != EntityExtractorStageID {
			t.Errorf("StageError.Stage = %q, want %q", stageErr.Stage, EntityExtractorStageID)
		}
		if stageErr.Message != "expected []*pipeline.Candidate" {
			t.Errorf("StageError.Message = %q, want %q", stageErr.Message, "expected []*pipeline.Candidate")
		}
	}
}

func TestEntityExtractorStage_Process_DetectorWithNoRegister(t *testing.T) {
	detector := &fakeDetector{
		spans: []pipeline.EntitySpan{
			{Type: "PERSON", Value: "John", Start: 0, End: 4, Confidence: 0.95, Detector: "test"},
			{Type: "LOCATION", Value: "New York", Start: 19, End: 27, Confidence: 0.92, Detector: "test"},
		},
	}

	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   detector,
		register:   nil,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{
		{
			DocumentID: "doc1",
			Content:    "John Doe lives in New York",
			Metadata:   nil,
		},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	if len(outCandidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(outCandidates))
	}

	// Verify detector was called
	if detector.callCount != 1 {
		t.Errorf("detector.Detect call count = %d, want 1", detector.callCount)
	}

	// Verify entities are attached
	entities, ok := outCandidates[0].Metadata["entities"].([]pipeline.EntitySpan)
	if !ok {
		t.Fatal("entities should be present as []pipeline.EntitySpan")
	}

	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}

	// Verify spans are sorted by Start
	if entities[0].Start >= entities[1].Start {
		t.Error("entities should be sorted by Start")
	}

	// Verify entity_summary
	summary, ok := outCandidates[0].Metadata["entity_summary"].(map[pipeline.EntityType]int)
	if !ok {
		t.Fatal("entity_summary should be present as map[EntityType]int")
	}

	if summary[pipeline.EntityType("PERSON")] != 1 {
		t.Errorf("PERSON count = %d, want 1", summary[pipeline.EntityType("PERSON")])
	}
	if summary[pipeline.EntityType("LOCATION")] != 1 {
		t.Errorf("LOCATION count = %d, want 1", summary[pipeline.EntityType("LOCATION")])
	}
}

func TestEntityExtractorStage_Process_HallucinatedSpanDropped(t *testing.T) {
	detector := &fakeDetector{
		spans: []pipeline.EntitySpan{
			{Type: "PERSON", Value: "John", Start: 0, End: 4, Confidence: 0.95, Detector: "test"},
			{Type: "PERSON", Value: "NonExistent", Start: 100, End: 111, Confidence: 0.90, Detector: "test"},
			{Type: "LOCATION", Value: "New York", Start: 19, End: 27, Confidence: 0.92, Detector: "test"},
		},
	}

	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   detector,
		register:   nil,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{
		{
			DocumentID: "doc1",
			Content:    "John Doe lives in New York",
			Metadata:   nil,
		},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	entities, ok := outCandidates[0].Metadata["entities"].([]pipeline.EntitySpan)
	if !ok {
		t.Fatal("entities should be present")
	}

	// Only 2 valid spans should remain (hallucinated "NonExistent" dropped)
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities after filtering, got %d", len(entities))
	}

	// Verify the hallucinated span is not present
	for _, span := range entities {
		if span.Value == "NonExistent" {
			t.Fatal("hallucinated span should have been dropped")
		}
	}
}

func TestEntityExtractorStage_Process_OffsetDerived(t *testing.T) {
	// Detector provides wrong offsets; stage should recompute them
	detector := &fakeDetector{
		spans: []pipeline.EntitySpan{
			{Type: "PERSON", Value: "John", Start: 999, End: 999, Confidence: 0.95, Detector: "test"},
		},
	}

	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   detector,
		register:   nil,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{
		{
			DocumentID: "doc1",
			Content:    "John Doe lives here",
			Metadata:   nil,
		},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	entities, ok := outCandidates[0].Metadata["entities"].([]pipeline.EntitySpan)
	if !ok {
		t.Fatal("entities should be present")
	}

	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}

	// Verify offsets were recomputed correctly
	if entities[0].Start != 0 {
		t.Errorf("Start = %d, want 0", entities[0].Start)
	}
	if entities[0].End != 4 {
		t.Errorf("End = %d, want 4", entities[0].End)
	}

	// Verify the Value matches content slice
	if string(candidates[0].Content[entities[0].Start:entities[0].End]) != "John" {
		t.Error("content slice at offsets does not match span.Value")
	}
}

func TestEntityExtractorStage_Process_RegisterCacheHit(t *testing.T) {
	cachedSpans := []pipeline.EntitySpan{
		{Type: "EMAIL", Value: "test@example.com", Start: 50, End: 66, Confidence: 0.99, Detector: "cache"},
	}
	cachedAnalysis := &driven.EntityAnalysis{
		DocumentID:      "doc1",
		ContentSHA256:   "abc123",
		AnalyzerVersion: "entity-detector-v1",
		Spans:           cachedSpans,
	}

	detector := &fakeDetector{
		spans: []pipeline.EntitySpan{
			{Type: "PERSON", Value: "John", Start: 0, End: 4, Confidence: 0.95, Detector: "test"},
		},
	}

	register := &fakeRegister{
		getHit:      true,
		getAnalysis: cachedAnalysis,
	}

	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   detector,
		register:   register,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{
		{
			DocumentID: "doc1",
			Content:    "Some content with test@example.com in it",
			Metadata:   nil,
		},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	// Detector should NOT be called on cache hit
	if detector.callCount != 0 {
		t.Errorf("detector.Detect should not be called on cache hit, but was called %d times", detector.callCount)
	}

	// Register.Get should have been called
	if len(register.getCalls) != 1 {
		t.Errorf("register.Get call count = %d, want 1", len(register.getCalls))
	}

	// Verify cached spans are attached
	entities, ok := outCandidates[0].Metadata["entities"].([]pipeline.EntitySpan)
	if !ok {
		t.Fatal("entities should be present")
	}

	if len(entities) != 1 || entities[0].Type != "EMAIL" {
		t.Error("cached email span should be attached")
	}

	// Register.Put should NOT be called on cache hit
	if len(register.putCalls) != 0 {
		t.Errorf("register.Put should not be called on cache hit, but was called %d times", len(register.putCalls))
	}
}

func TestEntityExtractorStage_Process_RegisterCacheMiss_DetectorCalled_PutCalled(t *testing.T) {
	detector := &fakeDetector{
		spans: []pipeline.EntitySpan{
			{Type: "PERSON", Value: "Alice", Start: 0, End: 5, Confidence: 0.93, Detector: "test"},
		},
	}

	register := &fakeRegister{
		getHit: false, // cache miss
	}

	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   detector,
		register:   register,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{
		{
			DocumentID: "doc123",
			Content:    "Alice is here",
			Metadata:   nil,
		},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	// Detector should be called on cache miss
	if detector.callCount != 1 {
		t.Errorf("detector.Detect call count = %d, want 1", detector.callCount)
	}

	// Register.Put should be called with validated spans
	if len(register.putCalls) != 1 {
		t.Fatalf("register.Put call count = %d, want 1", len(register.putCalls))
	}

	putCall := register.putCalls[0]
	if putCall.DocumentID != "doc123" {
		t.Errorf("Put DocumentID = %q, want %q", putCall.DocumentID, "doc123")
	}
	if putCall.AnalyzerVersion != "entity-detector-v1" {
		t.Errorf("Put AnalyzerVersion = %q, want %q", putCall.AnalyzerVersion, "entity-detector-v1")
	}
	if len(putCall.Spans) != 1 {
		t.Fatalf("Put Spans count = %d, want 1", len(putCall.Spans))
	}

	// Verify entities are attached
	entities, ok := outCandidates[0].Metadata["entities"].([]pipeline.EntitySpan)
	if !ok {
		t.Fatal("entities should be present")
	}
	if len(entities) != 1 || entities[0].Type != "PERSON" {
		t.Error("detected PERSON span should be attached")
	}
}

func TestEntityExtractorStage_Process_RegisterGetError_FallThroughToDetector(t *testing.T) {
	detector := &fakeDetector{
		spans: []pipeline.EntitySpan{
			{Type: "PERSON", Value: "Bob", Start: 0, End: 3, Confidence: 0.91, Detector: "test"},
		},
	}

	register := &fakeRegister{
		getErr: errors.New("database connection failed"),
	}

	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   detector,
		register:   register,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{
		{
			DocumentID: "doc1",
			Content:    "Bob is here",
			Metadata:   nil,
		},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v, want nil (fail-soft)", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	// Detector should still be called (fall through on Get error)
	if detector.callCount != 1 {
		t.Errorf("detector.Detect call count = %d, want 1", detector.callCount)
	}

	// Entities should still be attached
	entities, ok := outCandidates[0].Metadata["entities"].([]pipeline.EntitySpan)
	if !ok {
		t.Fatal("entities should be present despite register error")
	}
	if len(entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(entities))
	}
}

func TestEntityExtractorStage_Process_DetectorError_FailSoft(t *testing.T) {
	detector := &fakeDetector{
		err: errors.New("detection service unavailable"),
	}

	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   detector,
		register:   nil,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{
		{
			DocumentID: "doc1",
			Content:    "Some content",
			Metadata:   map[string]any{"existing": "value"},
		},
		{
			DocumentID: "doc2",
			Content:    "More content",
			Metadata:   nil,
		},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v, want nil (fail-soft per candidate)", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	if len(outCandidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(outCandidates))
	}

	// First candidate should be skipped (no entities attached)
	if _, has := outCandidates[0].Metadata["entities"]; has {
		t.Error("entities should not be attached when detector errors")
	}
	// Existing metadata should be preserved
	if outCandidates[0].Metadata["existing"] != "value" {
		t.Error("existing metadata should be preserved even on detector error")
	}

	// Second candidate should also be skipped
	if _, has := outCandidates[1].Metadata["entities"]; has {
		t.Error("entities should not be attached for second candidate")
	}
}

func TestEntityExtractorStage_Process_RegisterPutError_SpansStillAttached(t *testing.T) {
	detector := &fakeDetector{
		spans: []pipeline.EntitySpan{
			{Type: "EMAIL", Value: "user@example.com", Start: 0, End: 16, Confidence: 0.97, Detector: "test"},
		},
	}

	register := &fakeRegister{
		getHit: false,
		putErr: errors.New("cache write failed"),
	}

	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   detector,
		register:   register,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{
		{
			DocumentID: "doc1",
			Content:    "Contact: user@example.com",
			Metadata:   nil,
		},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v, want nil (Put error is best-effort)", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	// Spans should still be attached despite Put error
	entities, ok := outCandidates[0].Metadata["entities"].([]pipeline.EntitySpan)
	if !ok {
		t.Fatal("entities should be attached despite register.Put error")
	}

	if len(entities) != 1 || entities[0].Type != "EMAIL" {
		t.Error("detected email span should be attached despite cache write failure")
	}

	summary, ok := outCandidates[0].Metadata["entity_summary"].(map[pipeline.EntityType]int)
	if !ok {
		t.Fatal("entity_summary should be attached despite register.Put error")
	}
	if summary[pipeline.EntityType("EMAIL")] != 1 {
		t.Error("entity_summary should be correct despite cache write failure")
	}
}

func TestEntityExtractorStage_Process_MultipleCandidates(t *testing.T) {
	detector := &fakeDetector{
		spans: []pipeline.EntitySpan{
			{Type: "PERSON", Value: "Alice", Start: 0, End: 5, Confidence: 0.93, Detector: "test"},
		},
	}

	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   detector,
		register:   nil,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{
		{
			DocumentID: "doc1",
			Content:    "Alice is here",
			Metadata:   nil,
		},
		{
			DocumentID: "doc2",
			Content:    "Alice is also there",
			Metadata:   nil,
		},
		{
			DocumentID: "doc3",
			Content:    "Someone else",
			Metadata:   nil,
		},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	if len(outCandidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(outCandidates))
	}

	// Each candidate should have entities processed independently
	for i := 0; i < 3; i++ {
		if outCandidates[i].DocumentID != candidates[i].DocumentID {
			t.Errorf("candidate %d DocumentID mismatch", i)
		}

		if _, has := outCandidates[i].Metadata["entities"]; !has {
			t.Errorf("candidate %d should have entities metadata", i)
		}
	}

	// Detector should be called 3 times (once per candidate, since register is nil)
	if detector.callCount != 3 {
		t.Errorf("detector.Detect call count = %d, want 3", detector.callCount)
	}
}

func TestEntityExtractorStage_Process_EmptyInput(t *testing.T) {
	detector := &fakeDetector{}

	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   detector,
		register:   nil,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	if len(outCandidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(outCandidates))
	}

	if detector.callCount != 0 {
		t.Errorf("detector should not be called for empty input")
	}
}

func TestEntityExtractorStage_Process_EntitySummaryCounts(t *testing.T) {
	detector := &fakeDetector{
		spans: []pipeline.EntitySpan{
			{Type: "PERSON", Value: "Alice", Start: 0, End: 5, Confidence: 0.93, Detector: "test"},
			{Type: "PERSON", Value: "Bob", Start: 10, End: 13, Confidence: 0.92, Detector: "test"},
			{Type: "EMAIL", Value: "alice@example.com", Start: 20, End: 37, Confidence: 0.98, Detector: "test"},
		},
	}

	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   detector,
		register:   nil,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{
		{
			DocumentID: "doc1",
			Content:    "Alice and Bob contact alice@example.com here",
			Metadata:   nil,
		},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	summary, ok := outCandidates[0].Metadata["entity_summary"].(map[pipeline.EntityType]int)
	if !ok {
		t.Fatal("entity_summary should be present")
	}

	if summary[pipeline.EntityType("PERSON")] != 2 {
		t.Errorf("PERSON count = %d, want 2", summary[pipeline.EntityType("PERSON")])
	}
	if summary[pipeline.EntityType("EMAIL")] != 1 {
		t.Errorf("EMAIL count = %d, want 1", summary[pipeline.EntityType("EMAIL")])
	}
}

func TestEntityExtractorStage_Process_SpansSortedByStart(t *testing.T) {
	detector := &fakeDetector{
		spans: []pipeline.EntitySpan{
			// Intentionally out of order
			{Type: "LOCATION", Value: "York", Start: 999, End: 999, Confidence: 0.90, Detector: "test"},
			{Type: "PERSON", Value: "John", Start: 999, End: 999, Confidence: 0.95, Detector: "test"},
			{Type: "LOCATION", Value: "New", Start: 999, End: 999, Confidence: 0.91, Detector: "test"},
		},
	}

	stage := &EntityExtractorStage{
		descriptor: NewEntityExtractorFactory().Descriptor(),
		detector:   detector,
		register:   nil,
		registry:   nil,
		logger:     slog.Default(),
	}

	candidates := []*pipeline.Candidate{
		{
			DocumentID: "doc1",
			Content:    "John lives in New York",
			Metadata:   nil,
		},
	}

	result, err := stage.Process(context.Background(), candidates)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	outCandidates, ok := result.([]*pipeline.Candidate)
	if !ok {
		t.Fatalf("result should be []*pipeline.Candidate, got %T", result)
	}

	entities, ok := outCandidates[0].Metadata["entities"].([]pipeline.EntitySpan)
	if !ok {
		t.Fatal("entities should be present")
	}

	if len(entities) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(entities))
	}

	// Verify they are sorted by Start ascending
	for i := 0; i < len(entities)-1; i++ {
		if entities[i].Start >= entities[i+1].Start {
			t.Errorf("entities not sorted: entities[%d].Start=%d >= entities[%d].Start=%d",
				i, entities[i].Start, i+1, entities[i+1].Start)
		}
	}

	// Verify expected order (John@0, New@14, York@18)
	if entities[0].Value != "John" || entities[0].Start != 0 {
		t.Errorf("first entity should be John at offset 0, got %q at offset %d", entities[0].Value, entities[0].Start)
	}
	if entities[1].Value != "New" || entities[1].Start != 14 {
		t.Errorf("second entity should be New at offset 14, got %q at offset %d", entities[1].Value, entities[1].Start)
	}
	if entities[2].Value != "York" || entities[2].Start != 18 {
		t.Errorf("third entity should be York at offset 18, got %q at offset %d", entities[2].Value, entities[2].Start)
	}
}

// --- Interface compliance tests ---

func TestEntityExtractorFactory_ImplementsStageFactory(t *testing.T) {
	var _ pipelineport.StageFactory = (*EntityExtractorFactory)(nil)
}

func TestEntityExtractorStage_ImplementsStage(t *testing.T) {
	var _ pipelineport.Stage = (*EntityExtractorStage)(nil)
}
