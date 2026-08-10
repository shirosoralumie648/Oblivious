package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"oblivious/server/internal/secretbox"
)

type SecretStorageFinding struct {
	Table         string                        `json:"table"`
	RecordID      string                        `json:"recordId"`
	Path          string                        `json:"path"`
	Domain        string                        `json:"domain"`
	Status        secretbox.SecretStorageStatus `json:"status"`
	NeedsRotation bool                          `json:"needsRotation"`
	Message       string                        `json:"message,omitempty"`
}

func AuditSecretStorage(ctx context.Context, db DB) ([]SecretStorageFinding, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	var findings []SecretStorageFinding

	channelFindings, err := auditScalarSecretRows(ctx, db, secretAuditScalarSpec{
		Table:  "channels",
		ID:     "id",
		Column: "api_key_encrypted",
		Domain: secretbox.DomainRelayChannelAPIKey,
	})
	if err != nil {
		return nil, err
	}
	findings = append(findings, channelFindings...)

	workflowFindings, err := auditJSONSecretRows(ctx, db, secretAuditJSONSpec{
		Table:       "workflows",
		ID:          "id",
		Column:      "definition",
		ColumnPath:  "definition",
		Domain:      secretbox.DomainWorkflowDefinitionSecretValue,
		IsSecretKey: isWorkflowSecretAuditKey,
	})
	if err != nil {
		return nil, err
	}
	findings = append(findings, workflowFindings...)

	publishingChannelFindings, err := auditJSONSecretRows(ctx, db, secretAuditJSONSpec{
		Table:       "channel_configs",
		ID:          "id",
		Column:      "config",
		ColumnPath:  "config",
		Domain:      secretbox.DomainPublishingChannelConfigKey,
		IsSecretKey: isGenericSecretAuditKey,
	})
	if err != nil {
		return nil, err
	}
	findings = append(findings, publishingChannelFindings...)

	workflowSnapshotFindings, err := auditJSONSecretRows(ctx, db, secretAuditJSONSpec{
		Table:       "workflow_executions",
		ID:          "id",
		Column:      "workflow_snapshot",
		ColumnPath:  "workflow_snapshot",
		Domain:      secretbox.DomainWorkflowDefinitionSecretValue,
		IsSecretKey: isWorkflowSecretAuditKey,
	})
	if err != nil {
		return nil, err
	}
	findings = append(findings, workflowSnapshotFindings...)

	workflowNodeFindings, err := auditWorkflowNodeExecutionSecrets(ctx, db)
	if err != nil {
		return nil, err
	}
	findings = append(findings, workflowNodeFindings...)

	alertFindings, err := auditJSONSecretRows(ctx, db, secretAuditJSONSpec{
		Table:       "observability_alert_provider_configs",
		ID:          "id",
		Column:      "config",
		ColumnPath:  "config",
		Domain:      secretbox.DomainObservabilityAlertProviderConfigKey,
		IsSecretKey: isGenericSecretAuditKey,
	})
	if err != nil {
		return nil, err
	}
	findings = append(findings, alertFindings...)

	return findings, nil
}

type secretAuditScalarSpec struct {
	Table  string
	ID     string
	Column string
	Domain string
}

type secretAuditJSONSpec struct {
	Table       string
	ID          string
	Column      string
	ColumnPath  string
	Domain      string
	IsSecretKey func(string) bool
}

func auditScalarSecretRows(ctx context.Context, db DB, spec secretAuditScalarSpec) ([]SecretStorageFinding, error) {
	rows, err := db.Query(ctx, fmt.Sprintf("SELECT %s, %s FROM %s", spec.ID, spec.Column, spec.Table))
	if err != nil {
		return nil, fmt.Errorf("query %s.%s secret storage: %w", spec.Table, spec.Column, err)
	}
	defer rows.Close()

	var findings []SecretStorageFinding
	for rows.Next() {
		var id string
		var stored string
		if err := rows.Scan(&id, &stored); err != nil {
			return nil, fmt.Errorf("scan %s.%s secret storage: %w", spec.Table, spec.Column, err)
		}
		inspection := secretbox.InspectStored(spec.Domain, stored)
		inspection.Path = spec.Column
		if inspection.NeedsRotation {
			findings = append(findings, findingFromInspection(spec.Table, id, inspection))
		}
	}
	return findings, nil
}

func auditJSONSecretRows(ctx context.Context, db DB, spec secretAuditJSONSpec) ([]SecretStorageFinding, error) {
	rows, err := db.Query(ctx, fmt.Sprintf("SELECT %s, %s FROM %s", spec.ID, spec.Column, spec.Table))
	if err != nil {
		return nil, fmt.Errorf("query %s.%s secret storage: %w", spec.Table, spec.Column, err)
	}
	defer rows.Close()

	var findings []SecretStorageFinding
	for rows.Next() {
		var id string
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, fmt.Errorf("scan %s.%s secret storage: %w", spec.Table, spec.Column, err)
		}
		var payload map[string]any
		if strings.TrimSpace(raw) == "" {
			payload = map[string]any{}
		} else if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return nil, fmt.Errorf("decode %s.%s secret storage for %s: %w", spec.Table, spec.Column, id, err)
		}
		for _, inspection := range secretbox.InspectStoredMap(spec.Domain, payload, spec.IsSecretKey) {
			if !inspection.NeedsRotation {
				continue
			}
			if spec.ColumnPath != "" {
				inspection.Path = spec.ColumnPath + "." + inspection.Path
			}
			findings = append(findings, findingFromInspection(spec.Table, id, inspection))
		}
	}
	return findings, nil
}

func auditWorkflowNodeExecutionSecrets(ctx context.Context, db DB) ([]SecretStorageFinding, error) {
	rows, err := db.Query(ctx, "SELECT id, input, context FROM workflow_node_executions")
	if err != nil {
		return nil, fmt.Errorf("query workflow_node_executions secret storage: %w", err)
	}
	defer rows.Close()

	var findings []SecretStorageFinding
	for rows.Next() {
		var id string
		var inputRaw string
		var contextRaw string
		if err := rows.Scan(&id, &inputRaw, &contextRaw); err != nil {
			return nil, fmt.Errorf("scan workflow_node_executions secret storage: %w", err)
		}
		inputFindings, err := auditJSONPayloadSecrets(secretAuditJSONPayloadSpec{
			Table:       "workflow_node_executions",
			RecordID:    id,
			ColumnPath:  "input",
			Domain:      secretbox.DomainWorkflowDefinitionSecretValue,
			IsSecretKey: isWorkflowSecretAuditKey,
			Raw:         inputRaw,
		})
		if err != nil {
			return nil, err
		}
		findings = append(findings, inputFindings...)
		contextFindings, err := auditJSONPayloadSecrets(secretAuditJSONPayloadSpec{
			Table:       "workflow_node_executions",
			RecordID:    id,
			ColumnPath:  "context",
			Domain:      secretbox.DomainWorkflowDefinitionSecretValue,
			IsSecretKey: isWorkflowSecretAuditKey,
			Raw:         contextRaw,
		})
		if err != nil {
			return nil, err
		}
		findings = append(findings, contextFindings...)
	}
	return findings, nil
}

type secretAuditJSONPayloadSpec struct {
	Table       string
	RecordID    string
	ColumnPath  string
	Domain      string
	IsSecretKey func(string) bool
	Raw         string
}

func auditJSONPayloadSecrets(spec secretAuditJSONPayloadSpec) ([]SecretStorageFinding, error) {
	var payload map[string]any
	if strings.TrimSpace(spec.Raw) == "" {
		payload = map[string]any{}
	} else if err := json.Unmarshal([]byte(spec.Raw), &payload); err != nil {
		return nil, fmt.Errorf("decode %s.%s secret storage for %s: %w", spec.Table, spec.ColumnPath, spec.RecordID, err)
	}
	var findings []SecretStorageFinding
	for _, inspection := range secretbox.InspectStoredMap(spec.Domain, payload, spec.IsSecretKey) {
		if !inspection.NeedsRotation {
			continue
		}
		if spec.ColumnPath != "" {
			inspection.Path = spec.ColumnPath + "." + inspection.Path
		}
		findings = append(findings, findingFromInspection(spec.Table, spec.RecordID, inspection))
	}
	return findings, nil
}

func findingFromInspection(table, recordID string, inspection secretbox.SecretStorageInspection) SecretStorageFinding {
	return SecretStorageFinding{
		Table:         table,
		RecordID:      recordID,
		Path:          inspection.Path,
		Domain:        inspection.Domain,
		Status:        inspection.Status,
		NeedsRotation: inspection.NeedsRotation,
		Message:       inspection.Message,
	}
}

func isWorkflowSecretAuditKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", ""))
	return normalized == "secret" || normalized == "webhooksecret"
}

func isGenericSecretAuditKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(key), "_", ""), "-", ""))
	return strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "privatekey") ||
		strings.Contains(normalized, "routingkey")
}
