package pipeline

// Cardinality describes the input/output relationship within a stage.
type Cardinality string

const (
	CardinalityOneToOne   Cardinality = "1:1" // One input produces one output
	CardinalityOneToMany  Cardinality = "1:N" // One input produces multiple outputs
	CardinalityManyToMany Cardinality = "N:N" // Multiple inputs produce multiple outputs
	CardinalityManyToOne  Cardinality = "N:1" // Multiple inputs produce one output
)

// ShapeName identifies the data type flowing between stages.
type ShapeName string

const (
	// Indexing shapes
	ShapeContent       ShapeName = "content"
	ShapeChunk         ShapeName = "chunk"
	ShapeEnrichedChunk ShapeName = "enriched_chunk"
	ShapeEmbeddedChunk ShapeName = "embedded_chunk"
	ShapeIndexedDoc    ShapeName = "indexed_doc"

	// Search shapes
	ShapeQuery           ShapeName = "query"
	ShapeParsedQuery     ShapeName = "parsed_query"
	ShapeQuerySet        ShapeName = "query_set"
	ShapeExpandedQuery   ShapeName = "expanded_query"
	ShapeCandidate       ShapeName = "candidate"
	ShapeRankedResult    ShapeName = "ranked_result"
	ShapePresentedResult ShapeName = "presented_result"
)

// CapabilityType identifies an external swappable infrastructure provider
// that a pipeline stage may consume. The capability registry is the runtime
// dependency-injection mechanism for these providers.
//
// What belongs here (capability):
//
//   - External services with multiple plausible vendor implementations:
//     LLM, embedder, search engine, vector store, ACL/document-ID provider.
//   - Things whose presence-vs-absence drives "skip stage" / "graceful
//     degrade" behavior in the pipeline builder.
//   - Things a deployment may swap by changing wiring at startup.
//
// What does NOT belong here (use constructor injection instead):
//
//   - Application-internal ports backed by the application's own database
//     (document store, source store, sync store, audit repositories, and
//     similar CRUD-shaped data ports). Wire these directly into a stage
//     factory's constructor at startup, the way DocumentStore is wired
//     into the rest of the application.
//   - Things consumed by exactly one stage with one expected implementation.
//     The capability registry's indirection adds DI ceremony but earns
//     nothing when there is no swappability story.
//   - Stage-specific logic dressed up as a separate provider. A wrapper
//     that only consumes an existing capability (e.g. an LLM-backed
//     "detector" or "summarizer" or "rewriter") is not a new capability —
//     it is stage logic that consumes the LLM capability. The
//     query-expander stage is the canonical example: it declares
//     CapabilityLLM (Optional) and runs the prompt inline. New LLM-backed
//     features should follow that pattern.
//
// Adding a new constant here is a meaningful act. Before doing so, confirm:
// (a) is this an external service with realistic vendor swappability, or
// (b) am I leaking a stage's internal data dependencies through the registry
// because the constant feels like it "belongs in the same place" as LLM and
// vector-store? If (b), use a constructor arg instead.
//
// Because CapabilityType is an open string type, downstream consumers can
// declare their own constants in their own packages (`pipeline.CapabilityType("foo")`).
// Constants declared here should be those that Core stages actually consume.
type CapabilityType string

const (
	CapabilityLLM                CapabilityType = "llm"
	CapabilityEmbedder           CapabilityType = "embedder"
	CapabilitySearchEngine       CapabilityType = "search_engine" // BM25/text search (driven.SearchEngine)
	CapabilityVectorStore        CapabilityType = "vector_store"  // Vector similarity (driven.VectorIndex)
	CapabilityGraphStore         CapabilityType = "graph_store"
	CapabilityDocStore           CapabilityType = "doc_store"
	CapabilityOntology           CapabilityType = "ontology"
	CapabilityDocumentIDProvider CapabilityType = "document_id_provider" // Provides allowed document IDs for filtering
)

// CapabilityMode describes how a stage depends on a capability.
type CapabilityMode string

const (
	CapabilityRequired CapabilityMode = "required" // Stage fails without it
	CapabilityOptional CapabilityMode = "optional" // Stage degrades gracefully
	CapabilityFallback CapabilityMode = "fallback" // Used if primary unavailable
)

// StageType categorizes stages by their function.
type StageType string

const (
	// Indexing stage types
	StageTypeTransformer StageType = "transformer" // Modify/split content
	StageTypeEnricher    StageType = "enricher"    // Add metadata/embeddings
	StageTypeLoader      StageType = "loader"      // Persist to stores

	// Search stage types
	StageTypeParser    StageType = "parser"    // Parse raw query
	StageTypeExpander  StageType = "expander"  // Expand query terms
	StageTypeRetriever StageType = "retriever" // Fetch candidates
	StageTypeRanker    StageType = "ranker"    // Score/reorder results
	StageTypePresenter StageType = "presenter" // Format final output
)

// PipelineType identifies whether a pipeline is for indexing or search.
type PipelineType string

const (
	PipelineTypeIndexing PipelineType = "indexing"
	PipelineTypeSearch   PipelineType = "search"
)
