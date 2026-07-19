package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"oblivious/server/internal/config"
	"oblivious/server/internal/releasecontract"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

const deploymentReadinessProbeTimeout = 5 * time.Second

type deploymentReadinessProbe struct {
	id           string
	dependencyID string
	run          func(context.Context) error
}

func (p deploymentReadinessProbe) ID() string           { return p.id }
func (p deploymentReadinessProbe) DependencyID() string { return p.dependencyID }

func (p deploymentReadinessProbe) Run(parent context.Context) (releasecontract.Observation, error) {
	if parent == nil || p.run == nil {
		return blockedDependencyObservation(), nil
	}
	ctx, cancel := context.WithTimeout(parent, deploymentReadinessProbeTimeout)
	defer cancel()
	if err := p.run(ctx); err != nil || ctx.Err() != nil {
		return blockedDependencyObservation(), nil
	}
	return releasecontract.Observation{Availability: releasecontract.AvailabilityEnabled}, nil
}

func blockedDependencyObservation() releasecontract.Observation {
	return releasecontract.Observation{
		Availability: releasecontract.AvailabilityBlocked,
		ReasonCode:   "dependency_unproven",
	}
}

func newDeploymentReadinessProbes(cfg config.Config, database *sql.DB) ([]releasecontract.Probe, error) {
	return newDeploymentReadinessProbesWithOperations(cfg, database, defaultDeploymentReadinessOperations())
}

type deploymentReadinessOperations struct {
	postgres   func(context.Context, *sql.DB) error
	redis      func(context.Context, config.Config) error
	qdrant     func(context.Context, config.Config, string) error
	clickhouse func(context.Context, config.Config) error
	kafka      func(context.Context, []string) error
}

func defaultDeploymentReadinessOperations() deploymentReadinessOperations {
	return deploymentReadinessOperations{
		postgres: func(ctx context.Context, database *sql.DB) error {
			if database == nil {
				return fmt.Errorf("unconfigured")
			}
			return database.PingContext(ctx)
		},
		redis: func(ctx context.Context, cfg config.Config) error {
			if strings.TrimSpace(cfg.RedisAddr) == "" {
				return fmt.Errorf("unconfigured")
			}
			client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB, Protocol: 2})
			defer client.Close()
			return client.Ping(ctx).Err()
		},
		qdrant: func(ctx context.Context, cfg config.Config, qdrantHealthURL string) error {
			if qdrantHealthURL == "" {
				return fmt.Errorf("unconfigured")
			}
			transport := http.DefaultTransport.(*http.Transport).Clone()
			defer transport.CloseIdleConnections()
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, qdrantHealthURL, nil)
			if err != nil {
				return err
			}
			if cfg.QdrantAPIKey != "" {
				request.Header.Set("api-key", cfg.QdrantAPIKey)
			}
			response, err := (&http.Client{Transport: transport}).Do(request)
			if err != nil {
				return err
			}
			defer response.Body.Close()
			if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
				return fmt.Errorf("unhealthy")
			}
			return nil
		},
		clickhouse: func(ctx context.Context, cfg config.Config) error {
			if strings.TrimSpace(cfg.ClickHouseDSN) == "" || strings.TrimSpace(cfg.ClickHouseDriver) == "" {
				return fmt.Errorf("unconfigured")
			}
			client, err := sql.Open(cfg.ClickHouseDriver, cfg.ClickHouseDSN)
			if err != nil {
				return err
			}
			defer client.Close()
			return client.PingContext(ctx)
		},
		kafka: func(ctx context.Context, brokers []string) error {
			if len(brokers) == 0 {
				return fmt.Errorf("unconfigured")
			}
			var lastErr error
			for _, broker := range brokers {
				dialer := kafka.Dialer{Timeout: deploymentReadinessProbeTimeout, DualStack: true}
				connection, err := dialer.DialContext(ctx, "tcp", broker)
				if err != nil {
					lastErr = err
					continue
				}
				if deadline, ok := ctx.Deadline(); ok {
					_ = connection.SetDeadline(deadline)
				}
				_, metadataErr := connection.ReadPartitions()
				closeErr := connection.Close()
				if metadataErr == nil && closeErr == nil {
					return nil
				}
				if metadataErr != nil {
					lastErr = metadataErr
				} else {
					lastErr = closeErr
				}
			}
			return lastErr
		},
	}
}

func newDeploymentReadinessProbesWithOperations(cfg config.Config, database *sql.DB, operations deploymentReadinessOperations) ([]releasecontract.Probe, error) {
	qdrantHealthURL, err := readinessHealthURL(cfg.QdrantURL)
	if err != nil {
		return nil, fmt.Errorf("construct qdrant readiness probe: invalid configuration")
	}
	if err := validateKafkaBrokers(cfg.KafkaBrokers); err != nil {
		return nil, err
	}

	probes := []releasecontract.Probe{
		deploymentReadinessProbe{id: "runtime.postgres", dependencyID: "postgres", run: func(ctx context.Context) error {
			return operations.postgres(ctx, database)
		}},
		deploymentReadinessProbe{id: "runtime.redis", dependencyID: "redis", run: func(ctx context.Context) error {
			return operations.redis(ctx, cfg)
		}},
		deploymentReadinessProbe{id: "runtime.qdrant", dependencyID: "qdrant", run: func(ctx context.Context) error {
			return operations.qdrant(ctx, cfg, qdrantHealthURL)
		}},
		deploymentReadinessProbe{id: "runtime.clickhouse", dependencyID: "clickhouse", run: func(ctx context.Context) error {
			return operations.clickhouse(ctx, cfg)
		}},
		deploymentReadinessProbe{id: "runtime.kafka", dependencyID: "kafka", run: func(ctx context.Context) error {
			return operations.kafka(ctx, cfg.KafkaBrokers)
		}},
	}
	return probes, nil
}

func readinessHealthURL(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	endpoint, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" {
		return "", fmt.Errorf("invalid endpoint")
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/healthz"
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return endpoint.String(), nil
}

func validateKafkaBrokers(brokers []string) error {
	for _, broker := range brokers {
		host, portRaw, err := net.SplitHostPort(strings.TrimSpace(broker))
		if err != nil || strings.TrimSpace(host) == "" {
			return fmt.Errorf("construct kafka readiness probe: invalid configuration")
		}
		port, err := strconv.Atoi(portRaw)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("construct kafka readiness probe: invalid configuration")
		}
	}
	return nil
}

func newStartupReadinessManager(_ context.Context, inputs config.ResolvedEntrypointInputs, cfg config.Config, database *sql.DB, auditPath string) (releasecontract.ReadinessManager, error) {
	if strings.TrimSpace(auditPath) == "" {
		return nil, fmt.Errorf("OBLIVIOUS_READINESS_AUDIT_PATH is required")
	}
	probes, err := newDeploymentReadinessProbes(cfg, database)
	if err != nil {
		return nil, err
	}
	return releasecontract.NewManager(inputs.Contract(), inputs.Identity(), inputs.Profile(), releasecontract.NewEvaluator(), releasecontract.NewSystemClock(), probes, deploymentReadinessProbeTimeout, releasecontract.NewAtomicReadinessSnapshotWriter(), auditPath)
}
