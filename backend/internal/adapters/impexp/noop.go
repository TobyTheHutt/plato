package impexp

import "context"

// NoopImportExport is a no-op import and export adapter.
type NoopImportExport struct{}

// NewNoopImportExport returns a no-op import and export adapter.
func NewNoopImportExport() *NoopImportExport {
	return &NoopImportExport{}
}

// Import accepts input without persisting any changes.
func (noop *NoopImportExport) Import(_ context.Context, _ []byte) error {
	return nil
}

// Export returns an empty JSON object payload.
func (noop *NoopImportExport) Export(_ context.Context) ([]byte, error) {
	return []byte("{}"), nil
}
