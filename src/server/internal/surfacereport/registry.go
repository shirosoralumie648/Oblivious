package surfacereport

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
)

type detailsSchema struct {
	typeOf   reflect.Type
	decode   func(json.RawMessage) error
	validate func(any) error
}

type DetailsRegistry struct {
	schemas map[string]detailsSchema
}

func NewDetailsRegistry() *DetailsRegistry {
	registry := &DetailsRegistry{schemas: make(map[string]detailsSchema)}
	registerFoundationDetails(registry)
	return registry
}

func RegisterDetails[T any](registry *DetailsRegistry, surfaceID string, validate func(T) error) error {
	if registry == nil || strings.TrimSpace(surfaceID) == "" || validate == nil {
		return reportError("evidence.details", nil)
	}
	if _, exists := registry.schemas[surfaceID]; exists {
		return reportError("evidence.details", nil)
	}
	var zero T
	typeOf := reflect.TypeOf(zero)
	if typeOf == nil {
		return reportError("evidence.details", nil)
	}
	registry.schemas[surfaceID] = detailsSchema{
		typeOf: typeOf,
		decode: func(raw json.RawMessage) error {
			var value T
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&value); err != nil {
				return reportError("evidence.details", err)
			}
			var trailing any
			if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
				return reportError("evidence.details", err)
			}
			if err := validate(value); err != nil {
				return reportError("evidence.details", err)
			}
			return nil
		},
		validate: func(value any) error {
			typed, ok := value.(T)
			if !ok {
				return reportError("evidence.details", nil)
			}
			if err := validate(typed); err != nil {
				return reportError("evidence.details", err)
			}
			return nil
		},
	}
	return nil
}

func (r *DetailsRegistry) ValidateDetails(surfaceID string, raw json.RawMessage) error {
	if r == nil || len(raw) == 0 || string(raw) == "null" {
		return reportError("evidence.details", nil)
	}
	schema, ok := r.schemas[surfaceID]
	if !ok {
		return reportError("evidence.details", nil)
	}
	return schema.decode(raw)
}

func (r *DetailsRegistry) MarshalDetails(surfaceID string, value any) (json.RawMessage, error) {
	if r == nil {
		return nil, reportError("evidence.details", nil)
	}
	schema, ok := r.schemas[surfaceID]
	if !ok || value == nil || reflect.TypeOf(value) != schema.typeOf {
		return nil, reportError("evidence.details", nil)
	}
	if err := schema.validate(value); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, reportError("evidence.details", err)
	}
	return encoded, nil
}
