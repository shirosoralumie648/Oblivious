package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type ServiceDB struct {
	Relay         *sql.DB
	Chat          *sql.DB
	Workflow      *sql.DB
	RAG           *sql.DB
	Agent         *sql.DB
	Billing       *sql.DB
	Marketplace   *sql.DB
	Admin         *sql.DB
	Channel       *sql.DB
	Task          *sql.DB
	Observability *sql.DB
}

func Open(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func OpenMulti(mode string, monolithURL string, serviceURLs map[string]string) (*ServiceDB, error) {
	switch mode {
	case "monolith":
		db, err := Open(monolithURL)
		if err != nil {
			return nil, err
		}
		return &ServiceDB{
			Relay:         db,
			Chat:          db,
			Workflow:      db,
			RAG:           db,
			Agent:         db,
			Billing:       db,
			Marketplace:   db,
			Admin:         db,
			Channel:       db,
			Task:          db,
			Observability: db,
		}, nil

	case "dual_write", "microservices":
		sdb := &ServiceDB{}
		var err error

		if sdb.Relay, err = openOrFallback(serviceURLs["relay"], monolithURL); err != nil {
			return nil, fmt.Errorf("relay db: %w", err)
		}
		if sdb.Chat, err = openOrFallback(serviceURLs["chat"], monolithURL); err != nil {
			return nil, fmt.Errorf("chat db: %w", err)
		}
		if sdb.Workflow, err = openOrFallback(serviceURLs["workflow"], monolithURL); err != nil {
			return nil, fmt.Errorf("workflow db: %w", err)
		}
		if sdb.RAG, err = openOrFallback(serviceURLs["rag"], monolithURL); err != nil {
			return nil, fmt.Errorf("rag db: %w", err)
		}
		if sdb.Agent, err = openOrFallback(serviceURLs["agent"], monolithURL); err != nil {
			return nil, fmt.Errorf("agent db: %w", err)
		}
		if sdb.Billing, err = openOrFallback(serviceURLs["billing"], monolithURL); err != nil {
			return nil, fmt.Errorf("billing db: %w", err)
		}
		if sdb.Marketplace, err = openOrFallback(serviceURLs["marketplace"], monolithURL); err != nil {
			return nil, fmt.Errorf("marketplace db: %w", err)
		}
		if sdb.Admin, err = openOrFallback(serviceURLs["admin"], monolithURL); err != nil {
			return nil, fmt.Errorf("admin db: %w", err)
		}
		if sdb.Channel, err = openOrFallback(serviceURLs["channel"], monolithURL); err != nil {
			return nil, fmt.Errorf("channel db: %w", err)
		}
		if sdb.Task, err = openOrFallback(serviceURLs["task"], monolithURL); err != nil {
			return nil, fmt.Errorf("task db: %w", err)
		}
		if sdb.Observability, err = openOrFallback(serviceURLs["observability"], monolithURL); err != nil {
			return nil, fmt.Errorf("observability db: %w", err)
		}

		return sdb, nil

	default:
		return nil, fmt.Errorf("invalid db mode: %q", mode)
	}
}

func openOrFallback(serviceURL, fallbackURL string) (*sql.DB, error) {
	if serviceURL != "" {
		return Open(serviceURL)
	}
	return Open(fallbackURL)
}
