package types

// OpenRetrieveImage is a browser-loadable image extracted from a retrieval chunk.
// Populated on POST /api/v1/open/knowledge/retrieve when resource_urls=public.
type OpenRetrieveImage struct {
	URL     string `json:"url"`
	Alt     string `json:"alt,omitempty"`
	ChunkID string `json:"chunk_id"`
}

// OpenRetrieveChunk is one open-retrieve hit with optional images for client UI.
type OpenRetrieveChunk struct {
	SearchResult
	Images []OpenRetrieveImage `json:"images,omitempty"`
}
