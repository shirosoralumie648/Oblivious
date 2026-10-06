package migration

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Validator 验证旧库和新库数据一致性
type Validator interface {
	Validate(ctx context.Context, legacyDB, newDB DB) error
}

// DB 抽象数据库接口
type DB interface {
	QueryRow(ctx context.Context, query string, args ...any) Row
	Query(ctx context.Context, query string, args ...any) (Rows, error)
}

type Row interface {
	Scan(dest ...any) error
}

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
}

func quoteIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// ValidateTableRowCount 比较表行数
func ValidateTableRowCount(ctx context.Context, legacyDB, newDB DB, tableName string) error {
	var legacyCount, newCount int64

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(tableName))

	if err := legacyDB.QueryRow(ctx, query).Scan(&legacyCount); err != nil {
		return fmt.Errorf("query legacy row count: %w", err)
	}

	if err := newDB.QueryRow(ctx, query).Scan(&newCount); err != nil {
		return fmt.Errorf("query new row count: %w", err)
	}

	if legacyCount != newCount {
		return fmt.Errorf("row count mismatch: legacy=%d, new=%d", legacyCount, newCount)
	}

	return nil
}

// ValidateTableChecksum 比较表 checksum
func ValidateTableChecksum(ctx context.Context, legacyDB, newDB DB, tableName string, pkColumn string) error {
	legacyHash, err := computeTableChecksum(ctx, legacyDB, tableName, pkColumn)
	if err != nil {
		return fmt.Errorf("compute legacy checksum: %w", err)
	}

	newHash, err := computeTableChecksum(ctx, newDB, tableName, pkColumn)
	if err != nil {
		return fmt.Errorf("compute new checksum: %w", err)
	}

	if legacyHash != newHash {
		return fmt.Errorf("checksum mismatch: legacy=%s, new=%s", legacyHash, newHash)
	}

	return nil
}

func computeTableChecksum(ctx context.Context, db DB, tableName string, pkColumn string) (string, error) {
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY %s", quoteIdentifier(tableName), quoteIdentifier(pkColumn))
	rows, err := db.Query(ctx, query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	h := md5.New()
	var data []string

	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return "", err
		}
		data = append(data, row)
	}

	sort.Strings(data)
	for _, d := range data {
		h.Write([]byte(d))
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
