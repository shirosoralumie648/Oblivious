package migration

import (
	"context"
	"testing"
)

func TestRelayValidator_Validate(t *testing.T) {
	ctx := context.Background()

	legacyDB := &mockDB{
		queryRowFunc: func(ctx context.Context, query string, args ...any) Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					if count, ok := dest[0].(*int64); ok {
						*count = 100
					}
					return nil
				},
			}
		},
	}

	newDB := &mockDB{
		queryRowFunc: func(ctx context.Context, query string, args ...any) Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					if count, ok := dest[0].(*int64); ok {
						*count = 100
					}
					return nil
				},
			}
		},
	}

	validator := &RelayValidator{}
	err := validator.Validate(ctx, legacyDB, newDB)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
