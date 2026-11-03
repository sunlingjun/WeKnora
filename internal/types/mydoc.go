package types

import "time"

const (
	MyDocNodeFolder = "folder"
	MyDocNodeFile   = "file"

	MyDocKindUpload     = "upload"
	MyDocKindAIArtifact = "ai_artifact"
	MyDocKindQASnapshot = "qa_snapshot"

	MyDocRoleViewer = "viewer"

	MyDocTicketPreview  = "preview"
	MyDocTicketDownload = "download"

	MyDocViewOwned  = "owned"
	MyDocViewShared = "shared"
)

// MyDocNode is a folder or file entry in a user's personal document hub.
type MyDocNode struct {
	ID          string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	OwnerUserID string     `json:"owner_user_id" gorm:"type:varchar(64);index"`
	ParentID    *string    `json:"parent_id" gorm:"type:varchar(36)"`
	Name        string     `json:"name" gorm:"type:varchar(512)"`
	Type        string     `json:"type" gorm:"type:varchar(16)"`
	Archived    bool       `json:"archived"`
	ArchivedAt  *time.Time `json:"archived_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (MyDocNode) TableName() string { return "mydoc_nodes" }

// MyDocFile holds storage metadata for a file node.
type MyDocFile struct {
	NodeID        string    `json:"node_id" gorm:"type:varchar(36);primaryKey"`
	Kind          string    `json:"kind" gorm:"type:varchar(32)"`
	StorageKey    string    `json:"storage_key"`
	Mime          string    `json:"mime" gorm:"type:varchar(255)"`
	Size          int64     `json:"size"`
	ContentHash   string    `json:"content_hash" gorm:"type:varchar(128)"`
	Version       int       `json:"version"`
	SourceAgent   string    `json:"source_agent" gorm:"type:varchar(128)"`
	SourceSession string    `json:"source_session" gorm:"type:varchar(128)"`
	Extra         JSON      `json:"extra" gorm:"type:jsonb;default:'{}'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (MyDocFile) TableName() string { return "mydoc_files" }

// MyDocShare grants viewer access to a document for another user.
type MyDocShare struct {
	ID            string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	DocID         string    `json:"doc_id" gorm:"type:varchar(36);index"`
	GranteeUserID string    `json:"grantee_user_id" gorm:"type:varchar(64);index"`
	Role          string    `json:"role" gorm:"type:varchar(16)"`
	CreatedAt     time.Time `json:"created_at"`
}

func (MyDocShare) TableName() string { return "mydoc_shares" }

// MyDocTicket is a short-lived preview/download grant persisted by token hash.
type MyDocTicket struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	DocID     string    `json:"doc_id" gorm:"type:varchar(36);index"`
	Purpose   string    `json:"purpose" gorm:"type:varchar(16)"`
	TokenHash string    `json:"-" gorm:"type:varchar(128)"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
	CreatedBy string    `json:"created_by" gorm:"type:varchar(64)"`
	CreatedAt time.Time `json:"created_at"`
}

func (MyDocTicket) TableName() string { return "mydoc_tickets" }

// MyDocMount is a P1 placeholder for mounting a doc into a knowledge base.
type MyDocMount struct {
	ID                  string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	DocID               string    `json:"doc_id" gorm:"type:varchar(36)"`
	TenantID            uint64    `json:"tenant_id"`
	KnowledgeBaseID     string    `json:"knowledge_base_id" gorm:"type:varchar(36)"`
	DerivedKnowledgeID  string    `json:"derived_knowledge_id" gorm:"type:varchar(36)"`
	IndexStatus         string    `json:"index_status" gorm:"type:varchar(16)"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (MyDocMount) TableName() string { return "mydoc_mounts" }

// MyDocTreeNode is a list/tree response item with optional file meta.
type MyDocTreeNode struct {
	MyDocNode
	File     *MyDocFile `json:"file,omitempty"`
	Children []*MyDocTreeNode `json:"children,omitempty"`
}
