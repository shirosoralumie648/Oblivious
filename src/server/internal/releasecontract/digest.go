package releasecontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

const CanonicalFormatV1 = "contract-canonical/v1"

// CanonicalBytes encodes a previously validated contract without mutating it.
func CanonicalBytes(contract AuthoredContractV1) ([]byte, error) {
	normalized, err := normalizedContract(contract)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal normalized release contract: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode normalized release contract: %w", err)
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("encode canonical release contract: %w", err)
	}
	return bytes.TrimSuffix(output.Bytes(), []byte{'\n'}), nil
}

func Digest(contract AuthoredContractV1) (string, error) {
	canonical, err := CanonicalBytes(contract)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func normalizedContract(contract AuthoredContractV1) (AuthoredContractV1, error) {
	// The JSON round trip gives the canonicalizer an isolated deep copy while
	// preserving the exact authored field set of the typed contract.
	raw, err := json.Marshal(contract)
	if err != nil {
		return AuthoredContractV1{}, fmt.Errorf("clone release contract: %w", err)
	}
	var normalized AuthoredContractV1
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return AuthoredContractV1{}, fmt.Errorf("decode cloned release contract: %w", err)
	}

	sort.Slice(normalized.Capabilities, func(i, j int) bool {
		return normalized.Capabilities[i].ID < normalized.Capabilities[j].ID
	})
	sort.Slice(normalized.ReasonCodes, func(i, j int) bool {
		return normalized.ReasonCodes[i].ID < normalized.ReasonCodes[j].ID
	})
	for i := range normalized.ReasonCodes {
		sort.Strings(normalized.ReasonCodes[i].AppliesTo)
	}
	sort.Slice(normalized.Profiles, func(i, j int) bool {
		return normalized.Profiles[i].ID < normalized.Profiles[j].ID
	})
	for i := range normalized.Profiles {
		profile := &normalized.Profiles[i]
		sort.Strings(profile.Topology.Components)
		sort.Strings(profile.Entrypoints)
		sort.Slice(profile.Dependencies, func(i, j int) bool {
			return profile.Dependencies[i].ID < profile.Dependencies[j].ID
		})
		sort.Slice(profile.StateStores, func(i, j int) bool {
			return profile.StateStores[i].ID < profile.StateStores[j].ID
		})
		sort.Slice(profile.CapabilityOverrides, func(i, j int) bool {
			return profile.CapabilityOverrides[i].CapabilityID < profile.CapabilityOverrides[j].CapabilityID
		})
		sort.Strings(profile.CatalogBindingIDs)
		sort.Strings(profile.SurfaceReferenceIDs)
		sort.Strings(profile.ReadinessRequirementIDs)
	}
	sort.Slice(normalized.CatalogBindings, func(i, j int) bool {
		return normalized.CatalogBindings[i].ID < normalized.CatalogBindings[j].ID
	})
	sort.Slice(normalized.SurfaceReferences, func(i, j int) bool {
		return normalized.SurfaceReferences[i].ID < normalized.SurfaceReferences[j].ID
	})
	for i := range normalized.SurfaceReferences {
		sort.Strings(normalized.SurfaceReferences[i].CapabilityIDs)
	}
	sort.Slice(normalized.ReadinessRequirements, func(i, j int) bool {
		return normalized.ReadinessRequirements[i].ID < normalized.ReadinessRequirements[j].ID
	})
	for i := range normalized.ReadinessRequirements {
		sort.Strings(normalized.ReadinessRequirements[i].CapabilityIDs)
		sort.Strings(normalized.ReadinessRequirements[i].DependencyIDs)
	}
	return normalized, nil
}
