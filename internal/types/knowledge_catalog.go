package types

import "time"

const (
	// CatalogAccessOwned marks a knowledge base that belongs to the caller's workspace.
	CatalogAccessOwned = "owned"
	// CatalogAccessOrgShare marks a knowledge base reached via organization kb_shares.
	CatalogAccessOrgShare = "org_share"

	// CatalogKBHardLimit is the P0 cap on knowledge bases returned in one catalog list.
	CatalogKBHardLimit = 2000
	// CatalogKnowledgeDefaultLimit is the default page size for catalog knowledge listing.
	CatalogKnowledgeDefaultLimit = 100
	// CatalogKnowledgeMaxLimit is the maximum page size for catalog knowledge listing.
	CatalogKnowledgeMaxLimit = 500
	// CatalogKnowledgeTypeFile is the REST knowledge type that may carry a source file.
	CatalogKnowledgeTypeFile = "file"
)

// ListCatalogKBsQuery filters GET /knowledge-catalog/knowledge-bases.
type ListCatalogKBsQuery struct {
	IncludeOrgShared bool
	Type             string
}

// KnowledgeCatalogCursorQuery is the exclusive (updated_at, id) cursor used by
// GET /knowledge-catalog/knowledge. UpdatedAfter is exclusive (updated_at >).
type KnowledgeCatalogCursorQuery struct {
	Limit           int
	ParseStatus     string
	UpdatedAfter    time.Time
	HasCursor       bool
	CursorUpdatedAt time.Time
	CursorID        string
}

// CatalogKnowledgeBase is one authorized knowledge base in the catalog.
type CatalogKnowledgeBase struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Description    string    `json:"description"`
	AccessSource   string    `json:"access_source"`
	OwnerTenantID  uint64    `json:"owner_tenant_id"`
	OrganizationID string    `json:"organization_id,omitempty"`
	ShareID        string    `json:"share_id,omitempty"`
	Permission     string    `json:"permission"`
	CanDownload    bool      `json:"can_download"`
	KnowledgeCount int64     `json:"knowledge_count"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CatalogKBListResult is the data payload of GET /knowledge-catalog/knowledge-bases.
type CatalogKBListResult struct {
	TenantID       uint64                 `json:"tenant_id"`
	GeneratedAt    time.Time              `json:"generated_at"`
	KnowledgeBases []CatalogKnowledgeBase `json:"knowledge_bases"`
	Total          int                    `json:"total"`
	Truncated      bool                   `json:"truncated"`
}

// ListCatalogKnowledgeQuery filters GET /knowledge-catalog/knowledge.
type ListCatalogKnowledgeQuery struct {
	KBID         string
	Limit        int
	Cursor       string
	UpdatedAfter time.Time
	ParseStatus  string
}

// CatalogKnowledgeItem is one knowledge metadata row. It must not expose
// file_path, vector_store_id, download tickets, or document body.
type CatalogKnowledgeItem struct {
	ID              string    `json:"id"`
	KnowledgeBaseID string    `json:"knowledge_base_id"`
	KnowledgeType   string    `json:"knowledge_type"`
	Title           string    `json:"title"`
	FileName        string    `json:"file_name"`
	FileType        string    `json:"file_type"`
	FileSize        int64     `json:"file_size"`
	FolderPath      string    `json:"folder_path"`
	ParseStatus     string    `json:"parse_status"`
	EnableStatus    string    `json:"enable_status"`
	Source          string    `json:"source"`
	HasFile         bool      `json:"has_file"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CatalogKnowledgeListResult is the data payload of GET /knowledge-catalog/knowledge.
type CatalogKnowledgeListResult struct {
	TenantID   uint64                 `json:"tenant_id"`
	KBID       string                 `json:"kb_id"`
	Items      []CatalogKnowledgeItem `json:"items"`
	NextCursor string                 `json:"next_cursor"`
	HasMore    bool                   `json:"has_more"`
}
