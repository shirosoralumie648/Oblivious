package migration

import (
	"context"
	"strings"
	"testing"

	"oblivious/server/internal/secretbox"
)

func TestAuditSecretStorageFindsPlaintextAndInvalidStoredSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", "test-secretbox-audit-key")

	protectedWorkflowSecret, err := secretbox.Protect(secretbox.DomainWorkflowDefinitionSecretValue, "whsec-protected")
	if err != nil {
		t.Fatalf("Protect workflow secret: %v", err)
	}
	protectedAlertSecret, err := secretbox.Protect(secretbox.DomainObservabilityAlertProviderConfigKey, "opsgenie-protected")
	if err != nil {
		t.Fatalf("Protect alert secret: %v", err)
	}

	db := &secretAuditDB{
		rows: map[string][]string{
			"channels": {
				"ch_plain|sk-legacy-channel",
				"ch_invalid|" + secretbox.CodecPrefix + "broken!",
			},
			"workflows": {
				`workflow_plain|{"webhook_secret":"whsec-legacy","nodes":[{"input":{"secret":"` + protectedWorkflowSecret + `"}}]}`,
			},
			"channel_configs": {
				`channel_plain|{"webhook_url":"https://hooks.example.test","secret":"publishing-legacy"}`,
			},
			"workflow_executions": {
				`wexec_plain|{"webhook_secret":"snapshot-legacy"}`,
			},
			"workflow_node_executions": {
				`wnode_plain|{"secret":"node-input-legacy"}|{"secret":"node-context-legacy"}`,
			},
			"observability_alert_provider_configs": {
				`alert_ok|{"api_key":"` + protectedAlertSecret + `"}`,
			},
		},
	}

	findings, err := AuditSecretStorage(context.Background(), db)
	if err != nil {
		t.Fatalf("AuditSecretStorage returned error: %v", err)
	}
	if len(findings) != 7 {
		t.Fatalf("expected seven rotation findings, got %+v", findings)
	}

	byKey := map[string]SecretStorageFinding{}
	for _, finding := range findings {
		byKey[finding.Table+"."+finding.RecordID+"."+finding.Path] = finding
		if strings.Contains(finding.Message, "sk-legacy-channel") ||
			strings.Contains(finding.Message, "whsec-legacy") ||
			strings.Contains(finding.Message, "publishing-legacy") ||
			strings.Contains(finding.Message, "snapshot-legacy") ||
			strings.Contains(finding.Message, "node-input-legacy") ||
			strings.Contains(finding.Message, "node-context-legacy") ||
			strings.Contains(finding.Message, "whsec-protected") ||
			strings.Contains(finding.Message, "opsgenie-protected") {
			t.Fatalf("finding leaked secret material: %+v", finding)
		}
	}
	if byKey["channels.ch_plain.api_key_encrypted"].Status != secretbox.SecretStorageStatusPlaintext {
		t.Fatalf("expected plaintext channel finding, got %+v", byKey["channels.ch_plain.api_key_encrypted"])
	}
	if byKey["channels.ch_invalid.api_key_encrypted"].Status != secretbox.SecretStorageStatusInvalidProtected {
		t.Fatalf("expected invalid protected channel finding, got %+v", byKey["channels.ch_invalid.api_key_encrypted"])
	}
	if byKey["workflows.workflow_plain.definition.webhook_secret"].Status != secretbox.SecretStorageStatusPlaintext {
		t.Fatalf("expected plaintext workflow finding, got %+v", byKey["workflows.workflow_plain.definition.webhook_secret"])
	}
	if byKey["channel_configs.channel_plain.config.secret"].Status != secretbox.SecretStorageStatusPlaintext {
		t.Fatalf("expected plaintext publishing channel finding, got %+v", byKey["channel_configs.channel_plain.config.secret"])
	}
	if byKey["workflow_executions.wexec_plain.workflow_snapshot.webhook_secret"].Status != secretbox.SecretStorageStatusPlaintext {
		t.Fatalf("expected plaintext workflow snapshot finding, got %+v", byKey["workflow_executions.wexec_plain.workflow_snapshot.webhook_secret"])
	}
	if byKey["workflow_node_executions.wnode_plain.input.secret"].Status != secretbox.SecretStorageStatusPlaintext {
		t.Fatalf("expected plaintext workflow node input finding, got %+v", byKey["workflow_node_executions.wnode_plain.input.secret"])
	}
	if byKey["workflow_node_executions.wnode_plain.context.secret"].Status != secretbox.SecretStorageStatusPlaintext {
		t.Fatalf("expected plaintext workflow node context finding, got %+v", byKey["workflow_node_executions.wnode_plain.context.secret"])
	}
	if _, ok := byKey["workflows.workflow_plain.definition.nodes[0].input.secret"]; ok {
		t.Fatalf("protected nested workflow secret should not be a rotation finding: %+v", byKey["workflows.workflow_plain.definition.nodes[0].input.secret"])
	}
	if _, ok := byKey["observability_alert_provider_configs.alert_ok.config.api_key"]; ok {
		t.Fatalf("protected alert provider secret should not be a rotation finding: %+v", byKey["observability_alert_provider_configs.alert_ok.config.api_key"])
	}
}

type secretAuditDB struct {
	rows map[string][]string
}

func (db *secretAuditDB) QueryRow(ctx context.Context, query string, args ...any) Row {
	return &mockRow{}
}

func (db *secretAuditDB) Query(ctx context.Context, query string, args ...any) (Rows, error) {
	for _, table := range []string{
		"workflow_node_executions",
		"workflow_executions",
		"observability_alert_provider_configs",
		"channel_configs",
		"workflows",
		"channels",
	} {
		if rows, ok := db.rows[table]; ok && strings.Contains(query, "FROM \""+table+"\"") {
			return &secretAuditRows{data: rows}, nil
		} else if rows, ok := db.rows[table]; ok && strings.Contains(query, "FROM "+table) {
			// Fallback for queries that don't quote identifiers (e.g. static ones like in auditWorkflowNodeExecutionSecrets)
			return &secretAuditRows{data: rows}, nil
		}
	}
	return &secretAuditRows{}, nil
}

type secretAuditRows struct {
	data  []string
	index int
}

func (r *secretAuditRows) Next() bool {
	r.index++
	return r.index <= len(r.data)
}

func (r *secretAuditRows) Scan(dest ...any) error {
	parts := strings.Split(r.data[r.index-1], "|")
	if str, ok := dest[0].(*string); ok {
		*str = parts[0]
	}
	for i := 1; i < len(dest); i++ {
		value := ""
		if len(parts) > i {
			value = parts[i]
		}
		if str, ok := dest[i].(*string); ok {
			*str = value
		}
	}
	return nil
}

func (r *secretAuditRows) Close() error {
	return nil
}
