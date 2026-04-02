package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"oblivious/server/internal/config"
	"oblivious/server/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	entries, err := os.ReadDir("migrations")
	if err != nil {
		log.Fatalf("read migrations dir: %v", err)
	}

	migrationPaths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		migrationPaths = append(migrationPaths, filepath.Join("migrations", entry.Name()))
	}
	sort.Strings(migrationPaths)

	for _, migrationPath := range migrationPaths {
		statement, err := os.ReadFile(migrationPath)
		if err != nil {
			log.Fatalf("read migration %s: %v", migrationPath, err)
		}

		if _, err := database.Exec(string(statement)); err != nil {
			log.Fatalf("apply migration %s: %v", migrationPath, err)
		}
	}

	fmt.Println("migrations applied")
}
