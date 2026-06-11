package migration

import (
	"context"
	"errors"
	"testing"
)

type mockDB struct {
	queryRowFunc func(ctx context.Context, query string, args ...any) Row
	queryFunc    func(ctx context.Context, query string, args ...any) (Rows, error)
}

func (m *mockDB) QueryRow(ctx context.Context, query string, args ...any) Row {
	return m.queryRowFunc(ctx, query, args...)
}

func (m *mockDB) Query(ctx context.Context, query string, args ...any) (Rows, error) {
	return m.queryFunc(ctx, query, args...)
}

type mockRow struct {
	scanFunc func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error {
	return m.scanFunc(dest...)
}

type mockRows struct {
	data  []string
	index int
}

func (m *mockRows) Next() bool {
	m.index++
	return m.index <= len(m.data)
}

func (m *mockRows) Scan(dest ...any) error {
	if m.index < 1 || m.index > len(m.data) {
		return errors.New("out of range")
	}
	if str, ok := dest[0].(*string); ok {
		*str = m.data[m.index-1]
	}
	return nil
}

func (m *mockRows) Close() error {
	return nil
}

func TestValidateTableRowCount(t *testing.T) {
	ctx := context.Background()

	t.Run("same count", func(t *testing.T) {
		legacyDB := &mockDB{
			queryRowFunc: func(ctx context.Context, query string, args ...any) Row {
				return &mockRow{scanFunc: func(dest ...any) error {
					if count, ok := dest[0].(*int64); ok {
						*count = 100
					}
					return nil
				}}
			},
		}
		newDB := &mockDB{
			queryRowFunc: func(ctx context.Context, query string, args ...any) Row {
				return &mockRow{scanFunc: func(dest ...any) error {
					if count, ok := dest[0].(*int64); ok {
						*count = 100
					}
					return nil
				}}
			},
		}

		err := ValidateTableRowCount(ctx, legacyDB, newDB, "users")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("different count", func(t *testing.T) {
		legacyDB := &mockDB{
			queryRowFunc: func(ctx context.Context, query string, args ...any) Row {
				return &mockRow{scanFunc: func(dest ...any) error {
					if count, ok := dest[0].(*int64); ok {
						*count = 100
					}
					return nil
				}}
			},
		}
		newDB := &mockDB{
			queryRowFunc: func(ctx context.Context, query string, args ...any) Row {
				return &mockRow{scanFunc: func(dest ...any) error {
					if count, ok := dest[0].(*int64); ok {
						*count = 90
					}
					return nil
				}}
			},
		}

		err := ValidateTableRowCount(ctx, legacyDB, newDB, "users")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("legacy query error", func(t *testing.T) {
		legacyDB := &mockDB{
			queryRowFunc: func(ctx context.Context, query string, args ...any) Row {
				return &mockRow{scanFunc: func(dest ...any) error {
					return errors.New("query failed")
				}}
			},
		}
		newDB := &mockDB{}

		err := ValidateTableRowCount(ctx, legacyDB, newDB, "users")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestValidateTableChecksum(t *testing.T) {
	ctx := context.Background()

	t.Run("same checksum", func(t *testing.T) {
		data := []string{"row1", "row2", "row3"}

		legacyDB := &mockDB{
			queryFunc: func(ctx context.Context, query string, args ...any) (Rows, error) {
				return &mockRows{data: data}, nil
			},
		}
		newDB := &mockDB{
			queryFunc: func(ctx context.Context, query string, args ...any) (Rows, error) {
				return &mockRows{data: data}, nil
			},
		}

		err := ValidateTableChecksum(ctx, legacyDB, newDB, "users", "id")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("different checksum", func(t *testing.T) {
		legacyDB := &mockDB{
			queryFunc: func(ctx context.Context, query string, args ...any) (Rows, error) {
				return &mockRows{data: []string{"row1", "row2"}}, nil
			},
		}
		newDB := &mockDB{
			queryFunc: func(ctx context.Context, query string, args ...any) (Rows, error) {
				return &mockRows{data: []string{"row1", "row3"}}, nil
			},
		}

		err := ValidateTableChecksum(ctx, legacyDB, newDB, "users", "id")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("legacy query error", func(t *testing.T) {
		legacyDB := &mockDB{
			queryFunc: func(ctx context.Context, query string, args ...any) (Rows, error) {
				return nil, errors.New("query failed")
			},
		}
		newDB := &mockDB{}

		err := ValidateTableChecksum(ctx, legacyDB, newDB, "users", "id")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}
