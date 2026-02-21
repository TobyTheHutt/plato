package impexp

import (
	"context"
	"testing"
)

func TestNoopImportExport(t *testing.T) {
	adapter := NewNoopImportExport()
	if err := adapter.Import(context.Background(), []byte("{}")); err != nil {
		t.Fatalf("unexpected import error: %v", err)
	}
	payload, err := adapter.Export(context.Background())
	if err != nil {
		t.Fatalf("unexpected export error: %v", err)
	}
	if string(payload) != "{}" {
		t.Fatalf("unexpected payload: %s", string(payload))
	}
}
