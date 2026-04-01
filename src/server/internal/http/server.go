package http

import (
	"fmt"
	stdhttp "net/http"
	"time"

	"oblivious/server/internal/config"
)

func NewServer(cfg config.Config) *stdhttp.Server {
	return &stdhttp.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}
}
