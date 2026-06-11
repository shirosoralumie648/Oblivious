package main

import (
	"context"
	"log"
	"net/http"
	"oblivious/server/internal/billing"
	"oblivious/server/pkg/config"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadBillingConfig()

	router := gin.Default()
	svc := billing.New(cfg)
	svc.RegisterRoutes(router)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down billing service...")
	srv.Shutdown(context.Background())
}
