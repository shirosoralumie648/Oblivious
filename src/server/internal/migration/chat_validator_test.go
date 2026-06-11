package migration

import (
	"context"
	"testing"
)

type mockDB struct {
	queryRowFunc func(ctx context.Context, query string, args ...any) Row
	queryFunc    func(ctx context.Context, query string, args ...any) (Rows, error)
}

func (m *mockDB) QueryRow(ctx context.Context, query string, args ...any) Row {
	if m.queryRowFunc != nil {
		return m.queryRowFunc(ctx, query, args...)
	}
	return &mockRow{}
}

func (m *mockDB) Query(ctx context.Context, query string, args ...any) (Rows, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, query, args...)
	}
	return &mockRows{}, nil
}

type mockRow struct {
	scanFunc func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	return nil
}

type mockRows struct {
	nextFunc  func() bool
	scanFunc  func(dest ...any) error
	closeFunc func() error
}

func (m *mockRows) Next() bool {
	if m.nextFunc != nil {
		return m.nextFunc()
	}
	return false
}

func (m *mockRows) Scan(dest ...any) error {
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	return nil
}

func (m *mockRows) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

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
