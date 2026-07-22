package http

import stdhttp "net/http"

func preferenceRouteSurfaceOperations() []OperationContractMetadataV1 {
	const capability = "identity.account_session"
	const response = "sha256:ff577012f7f8d53e8a0be43da0a5e3ec0cb0a3ca748287121bcb438c9d4c48bd"
	return []OperationContractMetadataV1{
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/app/me/preferences", "getPreferences", "cookie", capability, false, "", "200", response),
		routeSurfaceJSONOperation(stdhttp.MethodPut, "/api/v1/app/me/preferences", "updatePreferences", "cookie+csrf", capability, true, "#/components/schemas/UpdatePreferencesRequest", "200", response),
	}
}

func registerPreferenceRouteSurfaces(registrar *RouteSurfaceRegistrar, preferencesHandler preferencesHandler) error {
	operations := preferenceRouteSurfaceOperations()
	return registerRouteSurfaceBindings(registrar, []routeSurfaceBinding{
		{Operation: operations[0], Auth: RouteSurfaceAuthSession, Handler: stdhttp.HandlerFunc(preferencesHandler.get)},
		{Operation: operations[1], Auth: RouteSurfaceAuthSession, Handler: stdhttp.HandlerFunc(preferencesHandler.update)},
	})
}

func registerPreferenceRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, preferencesHandler preferencesHandler) {
	if err := registerPreferenceRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), preferencesHandler); err != nil {
		panic(err)
	}
}
