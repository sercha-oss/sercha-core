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
	ShapeExpandedQuery   ShapeName = "expanded_query"
	ShapeCandidate       ShapeName = "candidate"
	ShapeRankedResult    ShapeName = "ranked_result"
	ShapePresentedResult ShapeName = "presented_result"
)

// CapabilityType identifies external dependencies a stage may require.
type CapabilityType string

const (
	CapabilityLLM         CapabilityType = "llm"
	CapabilityEmbedder    CapabilityType = "embedder"
	CapabilityVectorStore CapabilityType = "vector_store"
	CapabilityGraphStore  CapabilityType = "graph_store"
	CapabilityDocStore    CapabilityType = "doc_store"
	CapabilityChunkStore  CapabilityType = "chunk_store"
	CapabilityOntology    CapabilityType = "ontology"
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
