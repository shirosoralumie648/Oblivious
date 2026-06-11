package migration

import (
	"context"
	"testing"
)

func TestChatValidator_Validate(t *testing.T) {
	ctx := context.Background()

	t.Run("matching row counts", func(t *testing.T) {
		legacyDB := &mockDB{
			queryRowFunc: func(ctx context.Context, query string, args ...any) Row {
				return &mockRow{
					scanFunc: func(dest ...any) error {
						if count, ok := dest[0].(*int64); ok {
							*count = 10
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
							*count = 10
						}
						return nil
					},
				}
			},
		}

		v := &ChatValidator{}
		if err := v.Validate(ctx, legacyDB, newDB); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}
