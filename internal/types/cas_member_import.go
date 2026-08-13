package types

// CASImportRow is one input line from Excel/CSV/pasted phones.
type CASImportRow struct {
	Row   int    `json:"row"`
	Phone string `json:"phone"`
	Name  string `json:"name"`
}

const (
	CASImportStatusImportable    = "importable"
	CASImportStatusAlreadyMember = "already_member"
	CASImportStatusNotFound      = "not_found"
	CASImportStatusNameMismatch  = "name_mismatch"
	CASImportStatusInvalidPhone  = "invalid_phone"
	CASImportStatusAmbiguous     = "ambiguous"
	CASImportStatusLocalConflict = "local_conflict"
	CASImportStatusFailed        = "failed"
	CASImportStatusSkipped       = "skipped"
	CASImportStatusImported      = "imported"

	CASImportActionCreateUser = "create_user"
	CASImportActionAddMember  = "add_member"
)

// CASImportPreviewRow is one classified row (preview or import result).
type CASImportPreviewRow struct {
	Row             int    `json:"row"`
	PhoneMasked     string `json:"phone_masked"`
	Name            string `json:"name"`
	CASUserID       string `json:"cas_user_id,omitempty"`
	CASRealName     string `json:"cas_real_name,omitempty"`
	CASLoginName    string `json:"cas_login_name,omitempty"`
	WeKnoraUserID   string `json:"weknora_user_id,omitempty"`
	WeKnoraExists   bool   `json:"weknora_exists"`
	AlreadyInTenant bool   `json:"already_in_tenant"`
	Action          string `json:"action,omitempty"`
	Status          string `json:"status"`
	Error           string `json:"error,omitempty"`
}

// CASImportPreview is the dry-run response.
type CASImportPreview struct {
	Total         int                   `json:"total"`
	Importable    int                   `json:"importable"`
	WillCreate    int                   `json:"will_create"`
	WillAdd       int                   `json:"will_add"`
	AlreadyMember int                   `json:"already_member"`
	NotFound      int                   `json:"not_found"`
	NameMismatch  int                   `json:"name_mismatch"`
	InvalidPhone  int                   `json:"invalid_phone"`
	Ambiguous     int                   `json:"ambiguous"`
	LocalConflict int                   `json:"local_conflict"`
	Failed        int                   `json:"failed"`
	Role          TenantRole            `json:"role"`
	Rows          []CASImportPreviewRow `json:"rows"`
}

// CASImportResult is the confirm-import response.
type CASImportResult struct {
	Total    int                   `json:"total"`
	Imported int                   `json:"imported"`
	Skipped  int                   `json:"skipped"`
	Failed   int                   `json:"failed"`
	Role     TenantRole            `json:"role"`
	Rows     []CASImportPreviewRow `json:"rows"`
}

const CASImportMaxRows = 200
