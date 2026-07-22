package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime"
	stdhttp "net/http"
	"sort"
	"strings"
)

type SchemaIdentityV1 struct {
	Kind  string `json:"kind"`
	Value string `json:"value,omitempty"`
}

type MediaSchemaIdentityV1 struct {
	MediaType      string           `json:"mediaType,omitempty"`
	SchemaIdentity SchemaIdentityV1 `json:"schemaIdentity"`
}

type StatusMediaSchemaIdentityV1 struct {
	Status         string           `json:"status"`
	MediaType      string           `json:"mediaType,omitempty"`
	SchemaIdentity SchemaIdentityV1 `json:"schemaIdentity"`
}

type OperationContractMetadataV1 struct {
	Method           string                        `json:"method"`
	NormalizedPath   string                        `json:"normalizedPath"`
	OperationID      string                        `json:"operationId"`
	Security         string                        `json:"security"`
	CSRF             bool                          `json:"csrf"`
	CapabilityID     string                        `json:"capabilityId"`
	Request          MediaSchemaIdentityV1         `json:"request"`
	SuccessResponses []StatusMediaSchemaIdentityV1 `json:"successResponses"`
}

type PublicOperationDispositionV1 struct {
	Method         string `json:"method"`
	NormalizedPath string `json:"normalizedPath"`
	Disposition    string `json:"disposition"`
	Reason         string `json:"reason"`
}

type PublicOperationScopeV1 struct {
	SchemaVersion     string                         `json:"schemaVersion"`
	MandatoryPrefixes []string                       `json:"mandatoryPrefixes"`
	Dispositions      []PublicOperationDispositionV1 `json:"dispositions"`
}

type RouteSurfaceAuth string

const (
	RouteSurfaceAuthPublic  RouteSurfaceAuth = "public"
	RouteSurfaceAuthSession RouteSurfaceAuth = "session"
	RouteSurfaceAuthAdmin   RouteSurfaceAuth = "admin"
	RouteSurfaceAuthBearer  RouteSurfaceAuth = "bearer"
)

type RouteSurfaceRegistration struct {
	Method           string
	Path             string
	OperationID      string
	MiddlewareIDs    []string
	Auth             RouteSurfaceAuth
	CSRF             bool
	GuardEffectID    string
	CapabilityID     string
	Request          MediaSchemaIdentityV1
	SuccessResponses []StatusMediaSchemaIdentityV1
	Handler          stdhttp.Handler
}

type RouteSurfaceDescriptor struct {
	Method           string                        `json:"method"`
	Path             string                        `json:"normalizedPath"`
	OperationID      string                        `json:"operationId"`
	MiddlewareIDs    []string                      `json:"middlewareIds"`
	Auth             RouteSurfaceAuth              `json:"auth"`
	Security         string                        `json:"security"`
	CSRF             bool                          `json:"csrf"`
	GuardEffectID    string                        `json:"guardEffectId,omitempty"`
	CapabilityID     string                        `json:"capabilityId"`
	Request          MediaSchemaIdentityV1         `json:"request"`
	SuccessResponses []StatusMediaSchemaIdentityV1 `json:"successResponses"`
}

type RouteSurfaceMiddleware func(stdhttp.Handler) stdhttp.Handler
type RouteSurfaceGuard func(effectID, capabilityID string, next stdhttp.Handler) stdhttp.Handler

type RouteSurfacePolicies struct {
	Auth                map[RouteSurfaceAuth]RouteSurfaceMiddleware
	Middleware          map[string]RouteSurfaceMiddleware
	CSRF                RouteSurfaceMiddleware
	Guard               RouteSurfaceGuard
	AllowedCapabilities map[string]struct{}
}

type RouteSurfaceRegistrar struct {
	mux             *stdhttp.ServeMux
	policies        RouteSurfacePolicies
	descriptors     map[string]RouteSurfaceDescriptor
	mounts          map[string]string
	dispatchBridges map[string]string
	bridgeHandlers  map[string]*routeSurfaceDispatchBridge
	fallbacks       map[string]string
}

type routeSurfaceDispatchBridge struct {
	routes   map[string]routeSurfaceBridgeRoute
	fallback stdhttp.Handler
}

type routeSurfaceBridgeRoute struct {
	path    string
	handler stdhttp.Handler
}

func NewRouteSurfaceRegistrar(mux *stdhttp.ServeMux, policies RouteSurfacePolicies) (*RouteSurfaceRegistrar, error) {
	if mux == nil {
		return nil, routeSurfaceError("route_surface_mux_missing", "mux")
	}
	if len(policies.AllowedCapabilities) == 0 {
		return nil, routeSurfaceError("route_surface_policy_missing", "allowedCapabilities")
	}
	return &RouteSurfaceRegistrar{
		mux:             mux,
		policies:        cloneRouteSurfacePolicies(policies),
		descriptors:     make(map[string]RouteSurfaceDescriptor),
		mounts:          make(map[string]string),
		dispatchBridges: make(map[string]string),
		bridgeHandlers:  make(map[string]*routeSurfaceDispatchBridge),
		fallbacks:       make(map[string]string),
	}, nil
}

func (r *RouteSurfaceRegistrar) Register(reg RouteSurfaceRegistration) error {
	if r == nil || r.mux == nil {
		return routeSurfaceError("route_surface_registrar_missing", "registrar")
	}
	descriptor, err := descriptorFromRegistration(reg, r.policies)
	if err != nil {
		return err
	}
	key := routeSurfaceKey(descriptor.Method, descriptor.Path)
	if _, exists := r.descriptors[key]; exists {
		return routeSurfaceError("route_surface_duplicate", key)
	}

	handler := r.composeHandler(descriptor, reg.Handler)

	pattern := descriptor.Method + " " + descriptor.Path
	if err := handleRouteSurfacePattern(r.mux, pattern, handler); err != nil {
		if routeSurfaceErrorCode(err) != "route_surface_mount_invalid" {
			return err
		}
		bridgePattern := descriptor.Method + " " + routeSurfaceWildcardFallback(descriptor.Path)
		if strings.HasSuffix(bridgePattern, " ") {
			return err
		}
		if bridge, exists := r.bridgeHandlers[bridgePattern]; exists {
			if bridgeErr := bridge.add(key, descriptor.Path, handler); bridgeErr != nil {
				return bridgeErr
			}
		} else {
			bridge := &routeSurfaceDispatchBridge{
				routes:   make(map[string]routeSurfaceBridgeRoute),
				fallback: r.routeSurfaceNotFoundHandler(descriptor),
			}
			if bridgeErr := bridge.add(key, descriptor.Path, handler); bridgeErr != nil {
				return bridgeErr
			}
			if bridgeErr := handleRouteSurfacePattern(r.mux, bridgePattern, bridge); bridgeErr != nil {
				return bridgeErr
			}
			r.bridgeHandlers[bridgePattern] = bridge
		}
		r.dispatchBridges[key] = bridgePattern
	}
	r.descriptors[key] = descriptor
	r.mounts[key] = pattern
	return nil
}

func (r *RouteSurfaceRegistrar) composeHandler(descriptor RouteSurfaceDescriptor, handler stdhttp.Handler) stdhttp.Handler {
	if descriptor.GuardEffectID != "" {
		handler = r.policies.Guard(descriptor.GuardEffectID, descriptor.CapabilityID, handler)
	}
	for index := len(descriptor.MiddlewareIDs) - 1; index >= 0; index-- {
		handler = r.policies.Middleware[descriptor.MiddlewareIDs[index]](handler)
	}
	if descriptor.CSRF {
		handler = r.policies.CSRF(handler)
	}
	if descriptor.Auth != RouteSurfaceAuthPublic {
		handler = r.policies.Auth[descriptor.Auth](handler)
	}
	return handler
}

func (r *RouteSurfaceRegistrar) Snapshot() []RouteSurfaceDescriptor {
	if r == nil || r.validateSnapshot() != nil {
		return nil
	}
	result := make([]RouteSurfaceDescriptor, 0, len(r.descriptors))
	for _, descriptor := range r.descriptors {
		result = append(result, cloneRouteSurfaceDescriptor(descriptor))
	}
	sort.Slice(result, func(i, j int) bool {
		return routeSurfaceKey(result[i].Method, result[i].Path) < routeSurfaceKey(result[j].Method, result[j].Path)
	})
	return result
}

func (r *RouteSurfaceRegistrar) mountedPatterns() map[string]string {
	result := make(map[string]string, len(r.mounts))
	for key, pattern := range r.mounts {
		result[key] = pattern
	}
	return result
}

func (r *RouteSurfaceRegistrar) validateSnapshot() error {
	if r == nil || len(r.descriptors) == 0 || len(r.mounts) == 0 {
		return routeSurfaceError("route_surface_inventory_empty", "registrations")
	}
	if len(r.descriptors) != len(r.mounts) {
		return routeSurfaceError("route_surface_mount_descriptor_mismatch", "count")
	}
	for key, descriptor := range r.descriptors {
		if r.mounts[key] != descriptor.Method+" "+descriptor.Path {
			return routeSurfaceError("route_surface_mount_descriptor_mismatch", key)
		}
	}
	for key := range r.mounts {
		if _, ok := r.descriptors[key]; !ok {
			return routeSurfaceError("route_surface_mount_descriptor_mismatch", key)
		}
	}
	for key, bridgePattern := range r.dispatchBridges {
		descriptor, ok := r.descriptors[key]
		bridge := r.bridgeHandlers[bridgePattern]
		if !ok || bridge == nil || !bridge.contains(key, descriptor.Path) || bridgePattern != descriptor.Method+" "+routeSurfaceWildcardFallback(descriptor.Path) {
			return routeSurfaceError("route_surface_mount_descriptor_mismatch", key)
		}
	}
	for bridgePattern, bridge := range r.bridgeHandlers {
		if bridge == nil || len(bridge.routes) == 0 {
			return routeSurfaceError("route_surface_mount_descriptor_mismatch", bridgePattern)
		}
		for key, route := range bridge.routes {
			descriptor, ok := r.descriptors[key]
			if !ok || route.path != descriptor.Path || r.dispatchBridges[key] != bridgePattern {
				return routeSurfaceError("route_surface_mount_descriptor_mismatch", key)
			}
		}
	}
	for pattern, key := range r.fallbacks {
		descriptor, ok := r.descriptors[key]
		if !ok || !routeSurfaceFallbackBelongsToDescriptor(pattern, descriptor) {
			return routeSurfaceError("route_surface_mount_descriptor_mismatch", pattern)
		}
	}
	return nil
}

func (r *RouteSurfaceRegistrar) routeSurfaceNotFoundHandler(owner RouteSurfaceDescriptor) stdhttp.Handler {
	handler := stdhttp.Handler(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	}))
	if owner.Auth != RouteSurfaceAuthPublic {
		handler = r.policies.Auth[owner.Auth](handler)
	}
	return handler
}

func (b *routeSurfaceDispatchBridge) add(key, path string, handler stdhttp.Handler) error {
	if b == nil || handler == nil || strings.TrimSpace(key) == "" || strings.TrimSpace(path) == "" {
		return routeSurfaceError("route_surface_mount_invalid", key)
	}
	if _, exists := b.routes[key]; exists {
		return routeSurfaceError("route_surface_duplicate", key)
	}
	b.routes[key] = routeSurfaceBridgeRoute{path: path, handler: handler}
	return nil
}

func (b *routeSurfaceDispatchBridge) contains(key, path string) bool {
	if b == nil {
		return false
	}
	route, ok := b.routes[key]
	return ok && route.path == path && route.handler != nil
}

func (b *routeSurfaceDispatchBridge) ServeHTTP(w stdhttp.ResponseWriter, request *stdhttp.Request) {
	var matched stdhttp.Handler
	for _, route := range b.routes {
		if !routeSurfacePathMatches(route.path, request.URL.Path) {
			continue
		}
		if matched != nil {
			b.fallback.ServeHTTP(w, request)
			return
		}
		matched = route.handler
	}
	if matched == nil {
		b.fallback.ServeHTTP(w, request)
		return
	}
	matched.ServeHTTP(w, request)
}

func routeSurfacePathMatches(pattern, requestPath string) bool {
	patternSegments := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	requestSegments := strings.Split(strings.TrimPrefix(requestPath, "/"), "/")
	if len(patternSegments) != len(requestSegments) {
		return false
	}
	for index, segment := range patternSegments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			if requestSegments[index] == "" {
				return false
			}
			continue
		}
		if segment != requestSegments[index] {
			return false
		}
	}
	return true
}

type HTTPRuntimeObservation struct {
	OperationCount  int      `json:"operationCount"`
	MountedCount    int      `json:"mountedCount"`
	DescriptorCount int      `json:"descriptorCount"`
	MediaProbeCount int      `json:"mediaProbeCount"`
	CoreDigest      string   `json:"coreDigest"`
	RuntimeDigest   string   `json:"runtimeDigest"`
	ParityResult    string   `json:"parityResult"`
	MismatchIDs     []string `json:"mismatchIds"`
}

func CompareRouteSurfaceSnapshot(scope PublicOperationScopeV1, expected []OperationContractMetadataV1, actual []RouteSurfaceDescriptor) (HTTPRuntimeObservation, error) {
	observation := HTTPRuntimeObservation{
		OperationCount: len(expected), MountedCount: len(actual), DescriptorCount: len(actual),
		MediaProbeCount: countRouteSurfaceMedia(expected), ParityResult: "fail", MismatchIDs: []string{},
	}
	if len(expected) == 0 || len(actual) == 0 {
		return observation, routeSurfaceError("route_surface_inventory_empty", "operations")
	}
	scopeIndex, err := indexRouteSurfaceScope(scope)
	if err != nil {
		return observation, err
	}
	expectedIndex, err := indexExpectedRouteSurfaces(expected, scopeIndex)
	if err != nil {
		return observation, err
	}
	actualIndex, err := indexActualRouteSurfaces(actual, scopeIndex)
	if err != nil {
		return observation, err
	}
	keys := make(map[string]struct{}, len(expectedIndex)+len(actualIndex))
	for key := range expectedIndex {
		keys[key] = struct{}{}
	}
	for key := range actualIndex {
		keys[key] = struct{}{}
	}
	for key := range keys {
		expectedOperation, expectedOK := expectedIndex[key]
		actualOperation, actualOK := actualIndex[key]
		if !expectedOK || !actualOK {
			observation.MismatchIDs = append(observation.MismatchIDs, key)
			continue
		}
		if err := compareRouteSurfaceOperation(expectedOperation, actualOperation); err != nil {
			observation.MismatchIDs = append(observation.MismatchIDs, expectedOperation.OperationID)
		}
	}
	sort.Strings(observation.MismatchIDs)
	observation.CoreDigest, err = routeSurfaceDigest(expected)
	if err != nil {
		return observation, err
	}
	runtimeProjection := make([]OperationContractMetadataV1, 0, len(actual))
	for _, descriptor := range actual {
		runtimeProjection = append(runtimeProjection, operationFromRouteSurfaceDescriptor(descriptor))
	}
	observation.RuntimeDigest, err = routeSurfaceDigest(runtimeProjection)
	if err != nil {
		return observation, err
	}
	if len(observation.MismatchIDs) != 0 || observation.CoreDigest != observation.RuntimeDigest {
		return observation, routeSurfaceError("route_surface_parity_failed", strings.Join(observation.MismatchIDs, ","))
	}
	observation.ParityResult = "pass"
	return observation, nil
}

type RouteSurfaceContractError struct {
	Code  string
	Field string
}

func (e *RouteSurfaceContractError) Error() string {
	if e.Field == "" {
		return e.Code
	}
	return e.Code + ": field=" + e.Field
}

func routeSurfaceError(code, field string) error {
	return &RouteSurfaceContractError{Code: code, Field: field}
}

func routeSurfaceErrorCode(err error) string {
	if contractErr, ok := err.(*RouteSurfaceContractError); ok {
		return contractErr.Code
	}
	return ""
}

func cloneRouteSurfacePolicies(policies RouteSurfacePolicies) RouteSurfacePolicies {
	cloned := policies
	cloned.Auth = make(map[RouteSurfaceAuth]RouteSurfaceMiddleware, len(policies.Auth))
	for id, middleware := range policies.Auth {
		cloned.Auth[id] = middleware
	}
	cloned.Middleware = make(map[string]RouteSurfaceMiddleware, len(policies.Middleware))
	for id, middleware := range policies.Middleware {
		cloned.Middleware[id] = middleware
	}
	if policies.AllowedCapabilities != nil {
		cloned.AllowedCapabilities = make(map[string]struct{}, len(policies.AllowedCapabilities))
		for id := range policies.AllowedCapabilities {
			cloned.AllowedCapabilities[id] = struct{}{}
		}
	}
	return cloned
}

func descriptorFromRegistration(reg RouteSurfaceRegistration, policies RouteSurfacePolicies) (RouteSurfaceDescriptor, error) {
	reg.Method = strings.ToUpper(strings.TrimSpace(reg.Method))
	reg.Path = strings.TrimSpace(reg.Path)
	reg.OperationID = strings.TrimSpace(reg.OperationID)
	reg.CapabilityID = strings.TrimSpace(reg.CapabilityID)
	reg.GuardEffectID = strings.TrimSpace(reg.GuardEffectID)
	if reg.Handler == nil || reg.Method == "" || reg.Path == "" || reg.OperationID == "" || reg.CapabilityID == "" {
		return RouteSurfaceDescriptor{}, routeSurfaceError("route_surface_registration_invalid", "required")
	}
	if !strings.HasPrefix(reg.Path, "/") || strings.ContainsAny(reg.Path, "?#") || !validRouteSurfaceMethod(reg.Method) {
		return RouteSurfaceDescriptor{}, routeSurfaceError("route_surface_registration_invalid", "methodPath")
	}
	security, err := routeSurfaceSecurity(reg.Auth, reg.CSRF)
	if err != nil {
		return RouteSurfaceDescriptor{}, err
	}
	if reg.Auth != RouteSurfaceAuthPublic && policies.Auth[reg.Auth] == nil {
		return RouteSurfaceDescriptor{}, routeSurfaceError("route_surface_policy_missing", "auth")
	}
	if reg.CSRF && (policies.CSRF == nil || isSafeMethod(reg.Method)) {
		return RouteSurfaceDescriptor{}, routeSurfaceError("route_surface_policy_missing", "csrf")
	}
	if reg.GuardEffectID != "" && policies.Guard == nil {
		return RouteSurfaceDescriptor{}, routeSurfaceError("route_surface_policy_missing", "guard")
	}
	if _, ok := policies.AllowedCapabilities[reg.CapabilityID]; !ok {
		return RouteSurfaceDescriptor{}, routeSurfaceError("route_surface_capability_unknown", reg.CapabilityID)
	}
	seenMiddleware := map[string]struct{}{}
	for _, id := range reg.MiddlewareIDs {
		if id == "" || policies.Middleware[id] == nil {
			return RouteSurfaceDescriptor{}, routeSurfaceError("route_surface_policy_missing", "middleware")
		}
		if _, exists := seenMiddleware[id]; exists {
			return RouteSurfaceDescriptor{}, routeSurfaceError("route_surface_duplicate", "middleware")
		}
		seenMiddleware[id] = struct{}{}
	}
	if err := validateMediaSchemaIdentity(reg.Request); err != nil {
		return RouteSurfaceDescriptor{}, err
	}
	if len(reg.SuccessResponses) == 0 {
		return RouteSurfaceDescriptor{}, routeSurfaceError("route_surface_registration_invalid", "successResponses")
	}
	responses, err := normalizeRouteSurfaceResponses(reg.SuccessResponses)
	if err != nil {
		return RouteSurfaceDescriptor{}, err
	}
	return RouteSurfaceDescriptor{
		Method: reg.Method, Path: reg.Path, OperationID: reg.OperationID,
		MiddlewareIDs: append([]string(nil), reg.MiddlewareIDs...), Auth: reg.Auth, Security: security,
		CSRF: reg.CSRF, GuardEffectID: reg.GuardEffectID, CapabilityID: reg.CapabilityID,
		Request: reg.Request, SuccessResponses: responses,
	}, nil
}

func routeSurfaceSecurity(auth RouteSurfaceAuth, csrf bool) (string, error) {
	switch auth {
	case RouteSurfaceAuthPublic:
		if !csrf {
			return "public", nil
		}
	case RouteSurfaceAuthSession, RouteSurfaceAuthAdmin:
		if csrf {
			return "cookie+csrf", nil
		}
		return "cookie", nil
	case RouteSurfaceAuthBearer:
		if !csrf {
			return "bearer", nil
		}
	}
	return "", routeSurfaceError("route_surface_registration_invalid", "security")
}

func validRouteSurfaceMethod(method string) bool {
	switch method {
	case stdhttp.MethodGet, stdhttp.MethodHead, stdhttp.MethodPost, stdhttp.MethodPut, stdhttp.MethodPatch, stdhttp.MethodDelete, stdhttp.MethodOptions:
		return true
	default:
		return false
	}
}

func handleRouteSurfacePattern(mux *stdhttp.ServeMux, pattern string, handler stdhttp.Handler) (err error) {
	defer func() {
		if recover() != nil {
			err = routeSurfaceError("route_surface_mount_invalid", pattern)
		}
	}()
	mux.Handle(pattern, handler)
	return nil
}

var routeSurfaceFallbackMethods = []string{
	stdhttp.MethodConnect,
	stdhttp.MethodDelete,
	stdhttp.MethodGet,
	stdhttp.MethodOptions,
	stdhttp.MethodPatch,
	stdhttp.MethodPost,
	stdhttp.MethodPut,
	stdhttp.MethodTrace,
}

func (r *RouteSurfaceRegistrar) registerRoutingFallbacks() error {
	paths := make(map[string]map[string]string)
	for key, descriptor := range r.descriptors {
		if paths[descriptor.Path] == nil {
			paths[descriptor.Path] = make(map[string]string)
		}
		paths[descriptor.Path][descriptor.Method] = key
	}
	orderedPaths := make([]string, 0, len(paths))
	for path := range paths {
		orderedPaths = append(orderedPaths, path)
	}
	sort.Strings(orderedPaths)

	for _, path := range orderedPaths {
		methods := paths[path]
		ownerKey := routeSurfaceFallbackOwnerKey(methods)
		owner := r.descriptors[ownerKey]
		for _, method := range routeSurfaceFallbackMethods {
			if routeSurfacePathAllowsMethod(methods, method) {
				continue
			}
			pattern := method + " " + path
			if err := r.registerRoutingFallback(pattern, ownerKey, owner, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed"); err != nil {
				return err
			}
		}

		wildcardPath := routeSurfaceWildcardFallback(path)
		if wildcardPath == "" {
			continue
		}
		for _, method := range routeSurfaceFallbackMethods {
			pattern := method + " " + wildcardPath
			if err := r.registerRoutingFallback(pattern, ownerKey, owner, stdhttp.StatusNotFound, "not_found", "route not found"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RouteSurfaceRegistrar) registerRoutingFallback(pattern, ownerKey string, owner RouteSurfaceDescriptor, status int, code, message string) error {
	if _, exists := r.fallbacks[pattern]; exists {
		return nil
	}
	handler := stdhttp.Handler(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		writeError(w, status, code, message)
	}))
	if owner.Auth != RouteSurfaceAuthPublic {
		handler = r.policies.Auth[owner.Auth](handler)
	}
	if err := handleRouteSurfacePattern(r.mux, pattern, handler); err != nil {
		if routeSurfaceErrorCode(err) == "route_surface_mount_invalid" {
			return nil
		}
		return err
	}
	r.fallbacks[pattern] = ownerKey
	return nil
}

func routeSurfaceFallbackOwnerKey(methods map[string]string) string {
	keys := make([]string, 0, len(methods))
	for _, key := range methods {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys[0]
}

func routeSurfacePathAllowsMethod(methods map[string]string, method string) bool {
	if _, ok := methods[method]; ok {
		return true
	}
	_, getAllowed := methods[stdhttp.MethodGet]
	return method == stdhttp.MethodHead && getAllowed
}

func routeSurfaceFallbackBelongsToDescriptor(pattern string, descriptor RouteSurfaceDescriptor) bool {
	separator := strings.IndexByte(pattern, ' ')
	if separator <= 0 || separator == len(pattern)-1 {
		return false
	}
	path := pattern[separator+1:]
	return path == descriptor.Path || path == routeSurfaceWildcardFallback(descriptor.Path)
}

func routeSurfaceWildcardFallback(path string) string {
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for index, segment := range segments {
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") || strings.Contains(segment, "...") {
			continue
		}
		if index == 0 {
			return ""
		}
		return "/" + strings.Join(segments[:index], "/") + "/{routeSurfaceRemainder...}"
	}
	return ""
}

func cloneRouteSurfaceDescriptor(descriptor RouteSurfaceDescriptor) RouteSurfaceDescriptor {
	descriptor.MiddlewareIDs = append([]string(nil), descriptor.MiddlewareIDs...)
	descriptor.SuccessResponses = append([]StatusMediaSchemaIdentityV1(nil), descriptor.SuccessResponses...)
	return descriptor
}

func routeSurfaceKey(method, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(path)
}

func indexRouteSurfaceScope(scope PublicOperationScopeV1) (map[string]string, error) {
	if scope.SchemaVersion != "public-operation-scope/v1" || len(scope.Dispositions) == 0 {
		return nil, routeSurfaceError("route_surface_scope_invalid", "scope")
	}
	result := make(map[string]string, len(scope.Dispositions))
	for _, disposition := range scope.Dispositions {
		key := routeSurfaceKey(disposition.Method, disposition.NormalizedPath)
		if disposition.Disposition != "included" && disposition.Disposition != "excluded" {
			return nil, routeSurfaceError("route_surface_scope_invalid", key)
		}
		if _, exists := result[key]; exists {
			return nil, routeSurfaceError("route_surface_scope_duplicate", key)
		}
		result[key] = disposition.Disposition
	}
	return result, nil
}

func indexExpectedRouteSurfaces(expected []OperationContractMetadataV1, scope map[string]string) (map[string]OperationContractMetadataV1, error) {
	result := make(map[string]OperationContractMetadataV1, len(expected))
	for _, operation := range expected {
		key := routeSurfaceKey(operation.Method, operation.NormalizedPath)
		if scope[key] != "included" {
			return nil, routeSurfaceError("route_surface_expected_not_included", key)
		}
		if _, exists := result[key]; exists {
			return nil, routeSurfaceError("route_surface_duplicate", key)
		}
		result[key] = operation
	}
	return result, nil
}

func indexActualRouteSurfaces(actual []RouteSurfaceDescriptor, scope map[string]string) (map[string]RouteSurfaceDescriptor, error) {
	result := make(map[string]RouteSurfaceDescriptor, len(actual))
	for _, descriptor := range actual {
		key := routeSurfaceKey(descriptor.Method, descriptor.Path)
		if scope[key] == "excluded" {
			return nil, routeSurfaceError("route_surface_excluded_registered", key)
		}
		if scope[key] != "included" {
			return nil, routeSurfaceError("route_surface_runtime_unknown", key)
		}
		if _, exists := result[key]; exists {
			return nil, routeSurfaceError("route_surface_duplicate", key)
		}
		result[key] = descriptor
	}
	return result, nil
}

func compareRouteSurfaceOperation(expected OperationContractMetadataV1, actual RouteSurfaceDescriptor) error {
	if expected.OperationID != actual.OperationID || expected.Security != actual.Security || expected.CSRF != actual.CSRF || expected.CapabilityID != actual.CapabilityID {
		return routeSurfaceError("route_surface_metadata_mismatch", expected.OperationID)
	}
	if err := compareMediaSchemaIdentity(expected.Request, actual.Request); err != nil {
		return err
	}
	expectedResponses, err := normalizeRouteSurfaceResponses(expected.SuccessResponses)
	if err != nil {
		return err
	}
	actualResponses, err := normalizeRouteSurfaceResponses(actual.SuccessResponses)
	if err != nil {
		return err
	}
	if len(expectedResponses) != len(actualResponses) {
		return routeSurfaceError("route_surface_response_mismatch", expected.OperationID)
	}
	for index := range expectedResponses {
		if expectedResponses[index].Status != actualResponses[index].Status || expectedResponses[index].SchemaIdentity != actualResponses[index].SchemaIdentity || !compatibleRouteSurfaceMedia(expectedResponses[index].MediaType, actualResponses[index].MediaType) {
			return routeSurfaceError("route_surface_response_mismatch", expected.OperationID)
		}
	}
	return nil
}

func compareMediaSchemaIdentity(expected, actual MediaSchemaIdentityV1) error {
	if expected.SchemaIdentity != actual.SchemaIdentity || !compatibleRouteSurfaceMedia(expected.MediaType, actual.MediaType) {
		return routeSurfaceError("route_surface_request_mismatch", "request")
	}
	return nil
}

func compatibleRouteSurfaceMedia(expected, actual string) bool {
	expectedBase, expectedOK := routeSurfaceBaseMedia(expected)
	actualBase, actualOK := routeSurfaceBaseMedia(actual)
	return expectedOK && actualOK && expectedBase == actualBase
}

func routeSurfaceBaseMedia(value string) (string, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", true
	}
	base, _, err := mime.ParseMediaType(value)
	return strings.ToLower(base), err == nil
}

func validateMediaSchemaIdentity(identity MediaSchemaIdentityV1) error {
	if _, ok := routeSurfaceBaseMedia(identity.MediaType); !ok {
		return routeSurfaceError("route_surface_media_invalid", identity.MediaType)
	}
	switch identity.SchemaIdentity.Kind {
	case "none":
		if identity.SchemaIdentity.Value != "" {
			return routeSurfaceError("route_surface_schema_invalid", "none")
		}
	case "ref", "inline":
		if strings.TrimSpace(identity.SchemaIdentity.Value) == "" {
			return routeSurfaceError("route_surface_schema_invalid", identity.SchemaIdentity.Kind)
		}
	default:
		return routeSurfaceError("route_surface_schema_invalid", identity.SchemaIdentity.Kind)
	}
	return nil
}

func normalizeRouteSurfaceResponses(responses []StatusMediaSchemaIdentityV1) ([]StatusMediaSchemaIdentityV1, error) {
	result := append([]StatusMediaSchemaIdentityV1(nil), responses...)
	seen := map[string]struct{}{}
	for _, response := range result {
		if len(response.Status) != 3 || response.Status < "100" || response.Status > "599" {
			return nil, routeSurfaceError("route_surface_response_invalid", response.Status)
		}
		if err := validateMediaSchemaIdentity(MediaSchemaIdentityV1{MediaType: response.MediaType, SchemaIdentity: response.SchemaIdentity}); err != nil {
			return nil, err
		}
		mediaType, _ := routeSurfaceBaseMedia(response.MediaType)
		key := response.Status + "\x00" + mediaType + "\x00" + response.SchemaIdentity.Kind + "\x00" + response.SchemaIdentity.Value
		if _, exists := seen[key]; exists {
			return nil, routeSurfaceError("route_surface_duplicate", response.Status)
		}
		seen[key] = struct{}{}
	}
	sort.Slice(result, func(i, j int) bool {
		leftMedia, _ := routeSurfaceBaseMedia(result[i].MediaType)
		rightMedia, _ := routeSurfaceBaseMedia(result[j].MediaType)
		left := result[i].Status + "\x00" + leftMedia + "\x00" + result[i].SchemaIdentity.Kind + "\x00" + result[i].SchemaIdentity.Value
		right := result[j].Status + "\x00" + rightMedia + "\x00" + result[j].SchemaIdentity.Kind + "\x00" + result[j].SchemaIdentity.Value
		return left < right
	})
	return result, nil
}

func operationFromRouteSurfaceDescriptor(descriptor RouteSurfaceDescriptor) OperationContractMetadataV1 {
	return OperationContractMetadataV1{
		Method: descriptor.Method, NormalizedPath: descriptor.Path, OperationID: descriptor.OperationID,
		Security: descriptor.Security, CSRF: descriptor.CSRF, CapabilityID: descriptor.CapabilityID,
		Request: descriptor.Request, SuccessResponses: append([]StatusMediaSchemaIdentityV1(nil), descriptor.SuccessResponses...),
	}
}

func routeSurfaceDigest(operations []OperationContractMetadataV1) (string, error) {
	canonical := make([]OperationContractMetadataV1, 0, len(operations))
	for _, operation := range operations {
		normalized, err := canonicalRouteSurfaceOperation(operation)
		if err != nil {
			return "", err
		}
		canonical = append(canonical, normalized)
	}
	sort.Slice(canonical, func(i, j int) bool {
		return routeSurfaceKey(canonical[i].Method, canonical[i].NormalizedPath) < routeSurfaceKey(canonical[j].Method, canonical[j].NormalizedPath)
	})
	content, err := json.Marshal(canonical)
	if err != nil {
		return "", routeSurfaceError("route_surface_digest_failed", "operations")
	}
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func canonicalRouteSurfaceOperation(operation OperationContractMetadataV1) (OperationContractMetadataV1, error) {
	requestMedia, ok := routeSurfaceBaseMedia(operation.Request.MediaType)
	if !ok {
		return OperationContractMetadataV1{}, routeSurfaceError("route_surface_media_invalid", operation.Request.MediaType)
	}
	operation.Request.MediaType = requestMedia
	responses, err := normalizeRouteSurfaceResponses(operation.SuccessResponses)
	if err != nil {
		return OperationContractMetadataV1{}, err
	}
	for index := range responses {
		mediaType, ok := routeSurfaceBaseMedia(responses[index].MediaType)
		if !ok {
			return OperationContractMetadataV1{}, routeSurfaceError("route_surface_media_invalid", responses[index].MediaType)
		}
		responses[index].MediaType = mediaType
	}
	operation.SuccessResponses = responses
	return operation, nil
}

func countRouteSurfaceMedia(operations []OperationContractMetadataV1) int {
	count := 0
	for _, operation := range operations {
		if operation.Request.MediaType != "" {
			count++
		}
		for _, response := range operation.SuccessResponses {
			if response.MediaType != "" {
				count++
			}
		}
	}
	return count
}

func routeSurfaceRegistrationFromOperation(operation OperationContractMetadataV1, auth RouteSurfaceAuth, middlewareIDs []string, guardEffectID string, handler stdhttp.Handler) RouteSurfaceRegistration {
	return RouteSurfaceRegistration{
		Method: operation.Method, Path: operation.NormalizedPath, OperationID: operation.OperationID,
		MiddlewareIDs: append([]string(nil), middlewareIDs...), Auth: auth, CSRF: operation.CSRF,
		GuardEffectID: guardEffectID, CapabilityID: operation.CapabilityID, Request: operation.Request,
		SuccessResponses: append([]StatusMediaSchemaIdentityV1(nil), operation.SuccessResponses...), Handler: handler,
	}
}

func routeSurfaceMustOperation(method, path, operationID, security, capabilityID string, csrf bool, request MediaSchemaIdentityV1, responses ...StatusMediaSchemaIdentityV1) OperationContractMetadataV1 {
	return OperationContractMetadataV1{
		Method: method, NormalizedPath: path, OperationID: operationID, Security: security, CSRF: csrf,
		CapabilityID: capabilityID, Request: request, SuccessResponses: responses,
	}
}

func routeSurfaceNoneSchema() SchemaIdentityV1 { return SchemaIdentityV1{Kind: "none"} }
func routeSurfaceInlineSchema(digest string) SchemaIdentityV1 {
	return SchemaIdentityV1{Kind: "inline", Value: digest}
}
func routeSurfaceJSONResponse(status string, schema SchemaIdentityV1) StatusMediaSchemaIdentityV1 {
	return StatusMediaSchemaIdentityV1{Status: status, MediaType: "application/json", SchemaIdentity: schema}
}

var (
	routerOwnedAdminReadinessOperation = routeSurfaceMustOperation(
		stdhttp.MethodGet, "/api/v1/admin/readiness", "getAdminReadinessInventory", "cookie", "release.contract_reporting", false,
		MediaSchemaIdentityV1{SchemaIdentity: routeSurfaceNoneSchema()},
		routeSurfaceJSONResponse("200", routeSurfaceInlineSchema("sha256:a2c799262a3ce3c19ef5cdd983bf3d12b43ab3c426227091b909dcb7054738c0")),
	)
	routerOwnedAppReadinessOperation = routeSurfaceMustOperation(
		stdhttp.MethodGet, "/api/v1/app/readiness/capabilities", "getAppReadinessCapabilities", "cookie", "release.contract_reporting", false,
		MediaSchemaIdentityV1{SchemaIdentity: routeSurfaceNoneSchema()},
		routeSurfaceJSONResponse("200", routeSurfaceInlineSchema("sha256:a2c799262a3ce3c19ef5cdd983bf3d12b43ab3c426227091b909dcb7054738c0")),
	)
	routerOwnedWebSocketOperation = routeSurfaceMustOperation(
		stdhttp.MethodGet, "/api/v1/ws", "connectWorkspaceWebSocket", "cookie", "gateway.request_admission", false,
		MediaSchemaIdentityV1{SchemaIdentity: routeSurfaceNoneSchema()},
		StatusMediaSchemaIdentityV1{Status: "101", SchemaIdentity: routeSurfaceNoneSchema()},
	)
)

func routerOwnedRouteSurfaceOperations() []OperationContractMetadataV1 {
	return []OperationContractMetadataV1{
		routerOwnedAdminReadinessOperation,
		routerOwnedAppReadinessOperation,
		routerOwnedWebSocketOperation,
	}
}
