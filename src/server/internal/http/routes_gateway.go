package http

import stdhttp "net/http"

func gatewayRouteSurfaceOperations() []OperationContractMetadataV1 {
	return []OperationContractMetadataV1{
		routeSurfaceMustOperation(
			stdhttp.MethodPost,
			"/api/v1/gateway/proxy/chat/completions",
			"gatewayProxyCreateChatCompletion",
			"cookie+csrf",
			"relay.provider_inference",
			true,
			MediaSchemaIdentityV1{MediaType: "application/json", SchemaIdentity: SchemaIdentityV1{Kind: "ref", Value: "#/components/schemas/RelayOpenAIRequest"}},
			StatusMediaSchemaIdentityV1{Status: "200", MediaType: "application/json", SchemaIdentity: SchemaIdentityV1{Kind: "ref", Value: "#/components/schemas/RelayOpenAIResponse"}},
			StatusMediaSchemaIdentityV1{Status: "200", MediaType: "text/event-stream", SchemaIdentity: SchemaIdentityV1{Kind: "inline", Value: "sha256:00404e686415370f1711c4d7acfa2905444d3cf23cef2e10c47d445ebe690f96"}},
		),
	}
}

func registerGatewayRouteSurfaces(registrar *RouteSurfaceRegistrar, gatewayHandler gatewayHandler) error {
	return registerRouteSurfaceBindings(registrar, []routeSurfaceBinding{{
		Operation: gatewayRouteSurfaceOperations()[0],
		Auth:      RouteSurfaceAuthSession,
		Handler:   stdhttp.HandlerFunc(gatewayHandler.proxyChat),
	}})
}

func registerGatewayRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, gatewayHandler gatewayHandler) {
	if err := registerGatewayRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), gatewayHandler); err != nil {
		panic(err)
	}
}
