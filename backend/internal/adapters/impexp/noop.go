package impexp

import "context"

type NoopImportExport struct{}

func NewNoopImportExport() *NoopImportExport {
	return &NoopImportExport{}
}

func (noop *NoopImportExport) Import(_ context.Context, _ []byte) error {
	return nil
}

func (noop *NoopImportExport) Export(_ context.Context) ([]byte, error) {
	return []byte("{}"), nil
}
