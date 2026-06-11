package main

import (
	"context"
	"log"
	"net/http"
	"oblivious/server/internal/gateway"
	"oblivious/server/pkg/config"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadGatewayConfig()

	router := gin.Default()
	gw := gateway.New(cfg)
	gw.RegisterRoutes(router)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gateway...")
	srv.Shutdown(context.Background())
}
