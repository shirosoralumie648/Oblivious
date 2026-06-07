package http

import (
	"log"
	stdhttp "net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"oblivious/server/internal/workflow"
)

func workflowMetricsHandler(workflowService *workflow.Service) stdhttp.Handler {
	prometheusHandler := promhttp.Handler()
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if workflowService != nil {
			if err := workflowService.RefreshExecutionHealthMetrics(r.Context(), "", time.Now().UTC()); err != nil {
				log.Printf("warning: failed to refresh workflow execution health metrics: %v", err)
			}
		}
		prometheusHandler.ServeHTTP(w, r)
	})
}
