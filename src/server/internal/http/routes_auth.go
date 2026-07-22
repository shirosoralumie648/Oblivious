package http

import (
	stdhttp "net/http"
	"strings"
)

type routeSurfaceBinding struct {
	Operation     OperationContractMetadataV1
	Auth          RouteSurfaceAuth
	GuardEffectID string
	Handler       stdhttp.Handler
}

func registerRouteSurfaceBindings(registrar *RouteSurfaceRegistrar, bindings []routeSurfaceBinding) error {
	if registrar == nil || len(bindings) == 0 {
		return routeSurfaceError("route_surface_inventory_empty", "bindings")
	}
	for _, binding := range bindings {
		security, err := routeSurfaceSecurity(binding.Auth, binding.Operation.CSRF)
		if err != nil {
			return err
		}
		if security != binding.Operation.Security {
			return routeSurfaceError("route_surface_metadata_mismatch", binding.Operation.OperationID)
		}
		if err := registrar.Register(routeSurfaceRegistrationFromOperation(
			binding.Operation,
			binding.Auth,
			nil,
			binding.GuardEffectID,
			binding.Handler,
		)); err != nil {
			return err
		}
	}
	return registrar.registerRoutingFallbacks()
}

func routeSurfaceGroupAAllowedCapabilities() map[string]struct{} {
	return map[string]struct{}{
		"agent.run":                  {},
		"agent.tool_execution":       {},
		"chat.conversation_use":      {},
		"gateway.request_admission":  {},
		"identity.account_session":   {},
		"relay.provider_inference":   {},
		"release.contract_reporting": {},
		"task.scheduled_execution":   {},
	}
}

func mustRouteSurfaceAdapterRegistrar(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware) *RouteSurfaceRegistrar {
	pass := func(next stdhttp.Handler) stdhttp.Handler { return next }
	registrar, err := NewRouteSurfaceRegistrar(mux, RouteSurfacePolicies{
		Auth: map[RouteSurfaceAuth]RouteSurfaceMiddleware{
			RouteSurfaceAuthSession: authMiddleware.requireSession,
		},
		CSRF:                pass,
		AllowedCapabilities: routeSurfaceGroupAAllowedCapabilities(),
	})
	if err != nil {
		panic(err)
	}
	return registrar
}

func routeSurfaceJSONOperation(method, path, operationID, security, capabilityID string, csrf bool, requestRef, status, responseIdentity string) OperationContractMetadataV1 {
	request := MediaSchemaIdentityV1{SchemaIdentity: routeSurfaceNoneSchema()}
	if requestRef != "" {
		request = MediaSchemaIdentityV1{
			MediaType:      "application/json",
			SchemaIdentity: SchemaIdentityV1{Kind: "ref", Value: requestRef},
		}
	}
	responseSchema := SchemaIdentityV1{Kind: "inline", Value: responseIdentity}
	if strings.HasPrefix(responseIdentity, "#/") {
		responseSchema = SchemaIdentityV1{Kind: "ref", Value: responseIdentity}
	}
	return routeSurfaceMustOperation(
		method,
		path,
		operationID,
		security,
		capabilityID,
		csrf,
		request,
		routeSurfaceJSONResponse(status, responseSchema),
	)
}

func routeSurfaceNoContentOperation(method, path, operationID, security, capabilityID string, csrf bool) OperationContractMetadataV1 {
	return routeSurfaceMustOperation(
		method,
		path,
		operationID,
		security,
		capabilityID,
		csrf,
		MediaSchemaIdentityV1{SchemaIdentity: routeSurfaceNoneSchema()},
		StatusMediaSchemaIdentityV1{Status: "204", SchemaIdentity: routeSurfaceNoneSchema()},
	)
}

func authRouteSurfaceOperations() []OperationContractMetadataV1 {
	const capability = "identity.account_session"
	const sessionResponse = "sha256:86e766c97671bc4857a017e47cfa78c8cfa7dd0866e21e4732140856936cac32"
	return []OperationContractMetadataV1{
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/auth/login", "login", "public", capability, false, "#/components/schemas/CredentialsRequest", "200", sessionResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/auth/register", "register", "public", capability, false, "#/components/schemas/CredentialsRequest", "200", sessionResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/auth/password-reset/request", "requestPasswordReset", "public", capability, false, "#/components/schemas/PasswordResetRequest", "200", "sha256:2a690ffd90d3ad240fc1884302d45efd7d555b53c46c02e1069c0927c783607b"),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/auth/password-reset/confirm", "confirmPasswordReset", "public", capability, false, "#/components/schemas/PasswordResetConfirmRequest", "200", "sha256:e4c34ca855784761e3522ecccd9c43b4142acbe88b25ad4d50b9b489a8471d8e"),
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/auth/me", "getCurrentSession", "cookie", capability, false, "", "200", sessionResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/auth/logout", "logout", "cookie+csrf", capability, true, "", "200", "sha256:e57404f982552317c88e05254bb4120782173c24efe78f0a01f776bb9030e16e"),
	}
}

func registerAuthRouteSurfaces(registrar *RouteSurfaceRegistrar, authHandler authHandler) error {
	operations := authRouteSurfaceOperations()
	bindings := []routeSurfaceBinding{
		{Operation: operations[0], Auth: RouteSurfaceAuthPublic, Handler: stdhttp.HandlerFunc(authHandler.login)},
		{Operation: operations[1], Auth: RouteSurfaceAuthPublic, Handler: stdhttp.HandlerFunc(authHandler.register)},
		{Operation: operations[2], Auth: RouteSurfaceAuthPublic, Handler: stdhttp.HandlerFunc(authHandler.requestPasswordReset)},
		{Operation: operations[3], Auth: RouteSurfaceAuthPublic, Handler: stdhttp.HandlerFunc(authHandler.confirmPasswordReset)},
		{Operation: operations[4], Auth: RouteSurfaceAuthSession, Handler: stdhttp.HandlerFunc(authHandler.me)},
		{Operation: operations[5], Auth: RouteSurfaceAuthSession, Handler: stdhttp.HandlerFunc(authHandler.logout)},
	}
	return registerRouteSurfaceBindings(registrar, bindings)
}

// registerAuthRoutes remains for focused behavior tests and standalone routers.
// Production composition uses registerAuthRouteSurfaces with the shared registrar.
func registerAuthRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, authHandler authHandler) {
	if err := registerAuthRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), authHandler); err != nil {
		panic(err)
	}
}
