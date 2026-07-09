# Admin channel BaseURL SSRF guard

Date: 2026-07-04

## Scope

Closed a commercial P0 gateway/admin safety gap where an admin-created Relay channel could persist a provider `baseURL` targeting loopback, private, or link-local infrastructure. This is a local production-runtime guard for the Admin channel configuration surface; it does not replace target-runtime live provider evidence.

## Change

- `src/server/internal/admin/channel_service.go`
  - Validates channel `BaseURL` before `CreateChannel` reaches the store.
  - Validates channel `BaseURL` before `UpdateChannel` reaches the store.
  - Rejects local/private/link-local/multicast/unspecified IP hosts.
  - Rejects credentials in the URL.
  - Requires explicit configured channel base URLs to use `https`.
- `src/server/internal/admin/service_test.go`
  - Added `TestServiceRejectsUnsafeChannelBaseURLBeforePersisting`.
  - The regression proves loopback, RFC1918, and metadata-service URLs are rejected before persistence, relay config hot-apply, or audit logging.

## Verification

- RED: `cd src/server && go test ./internal/admin -run TestServiceRejectsUnsafeChannelBaseURLBeforePersisting -count=1 -v`
  - Failed because unsafe URLs were accepted: `got <nil>` for loopback/private/metadata cases.
- GREEN: `cd src/server && go test ./internal/admin -run 'TestServiceRejectsUnsafeChannelBaseURLBeforePersisting|TestServiceRedactsChannelAPIKeyFromAuditChanges|TestServicePassesChannelWeightThroughCreateAndUpdate|TestRelayConfigApplierRunsAfterChannelAndRouteMutations' -count=1 -v`
  - Passed.

## Remaining boundary

This guard only validates literal IP-host configured provider URLs at the Admin service layer. DNS rebinding/hostname resolution enforcement should still be handled by the outbound HTTP transport before enabling arbitrary custom upstream hosts in a hardened target deployment.
