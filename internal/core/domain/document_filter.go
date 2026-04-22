package domain

// DocumentIDFilter expresses an authoritative document-id filter with three-case semantics
// designed to close a fail-open hole in the permission-aware search path.
//
// A nil *DocumentIDFilter means "no filter" (the field is unset).
//
// A non-nil *DocumentIDFilter has two further cases controlled by Apply:
//
//   - Apply == false: no filter is applied. (Equivalent to a nil pointer; normalise
//     callers should prefer nil to express this.)
//   - Apply == true && len(IDs) == 0: authoritative deny-all. Downstream MUST return
//     zero results; adapters MUST NOT silently treat this as "no filter".
//   - Apply == true && len(IDs) > 0: authoritative allow-list. Downstream MUST restrict
//     results to exactly these IDs.
//
// This type exists specifically because bare []string conflates "no filter" and
// "deny all" — a provider that returns an empty slice for a user with access to zero
// documents would otherwise be silently granted access to every indexed document.
type DocumentIDFilter struct {
	Apply bool     `json:"apply"`
	IDs   []string `json:"ids"`
}

// DenyAllDocumentIDFilter returns a filter that matches zero documents.
func DenyAllDocumentIDFilter() *DocumentIDFilter {
	return &DocumentIDFilter{Apply: true, IDs: []string{}}
}

// AllowDocumentIDs returns a filter restricting results to the given IDs.
// If ids is empty, this is equivalent to DenyAllDocumentIDFilter.
func AllowDocumentIDs(ids []string) *DocumentIDFilter {
	if ids == nil {
		ids = []string{}
	}
	return &DocumentIDFilter{Apply: true, IDs: ids}
}

// IsDenyAll reports whether the filter, if applied, denies all documents.
// A nil receiver or Apply==false returns false (no filter means "no denial").
func (f *DocumentIDFilter) IsDenyAll() bool {
	return f != nil && f.Apply && len(f.IDs) == 0
}

// IsAllowList reports whether the filter is an active allow-list with at least one ID.
// A nil receiver or Apply==false returns false.
func (f *DocumentIDFilter) IsAllowList() bool {
	return f != nil && f.Apply && len(f.IDs) > 0
}
