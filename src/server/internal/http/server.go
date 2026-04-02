package http

import (
	"database/sql"
	"fmt"
	stdhttp "net/http"
	"time"

	"oblivious/server/internal/config"
)

func NewServer(cfg config.Config, database *sql.DB) *stdhttp.Server {
	return &stdhttp.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           NewRouter(cfg, database),
		ReadHeaderTimeout: 5 * time.Second,
	}
}
