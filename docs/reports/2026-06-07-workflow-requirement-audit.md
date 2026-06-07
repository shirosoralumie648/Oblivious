# Workflow Requirement Audit - 2026-06-07

Scope:

- `docs/superpowers/specs/2026-06-04-functional-logic-details.md` sections 2.1-2.6.
- `docs/superpowers/specs/2026-06-04-complete-fusion-design.md` section 3.2.
- `docs/superpowers/specs/2026-06-04-complete-fusion-design-part2.md` section 4.3.2.
- `docs/superpowers/specs/2026-06-04-complete-fusion-design-part3.md` section 8.3.3.

Status values:

- `Proven`: current code and focused tests prove the requirement.
- `Partial`: current code exists, but the requirement is only partly implemented or partly verified.
- `Gap`: current evidence contradicts or misses the requirement.
- `Unverified`: code may exist, but evidence is too weak for a completion claim.

## 2.1 Trigger Modes

| Requirement | Status | Evidence |
| --- | --- | --- |
| Manual execution with user input. | Proven | `Service.StartExecution` accepts `WorkflowTriggerManual` and input; `/api/v1/workflows/:id/execute` passes input. Covered by `TestServiceStartExecutionCreatesRunningExecutionFromWorkflow`, `TestWorkflowHandlerStartExecutionPassesInput`, and `TestWorkflowHandlerStartExecutionAutoModeRunsUntilBlocked`. |
| Conversation trigger binding `conversation_id -> workflow_id`. | Proven | `MatchConversationTriggers` scans published workflow trigger definitions, and Chat send-message now calls the workflow semantic/conversation dispatcher after a successful assistant reply. Covered by `TestServiceMatchConversationTriggersReturnsPublishedConversationBindings`, `TestWorkflowHandlerConversationMatchesRequireConversationID`, `TestRegisterWorkflowRoutesDispatchesConversationMatches`, `TestWorkflowSemanticTriggerDispatcherStartsMatchedWorkflows`, `TestSendMessageTriggersSemanticWorkflowsAfterAssistantReply`, and `TestDefaultChatServiceCanUseWorkflowSemanticTriggerDispatcher`. |
| Schedule trigger with cron expression. | Proven | `trigger.ScheduleTrigger` parses 5-field cron and computes next run; the default schedule service now registers itself as the workflow schedule syncer, so published workflow lifecycle changes sync schedule triggers to scheduled tasks. Covered by `TestServiceSyncsScheduleTriggersForPublishedWorkflowLifecycle`, `TestServiceCreatePublishedWorkflowSyncsScheduleTriggers`, `TestWorkflowServiceCanUseDefaultScheduleServiceForScheduleTriggerSync`, `TestRegisterWorkflowRoutesDispatchesWorkflowCrudAndTestNode`, and `TestDefaultRouterSyncsWorkflowScheduleTriggersToScheduledTasks`. |
| Webhook trigger with URL, secret, and payload. | Proven | `trigger.WebhookTrigger` validates HMAC signatures and methods; HTTP handler supports raw and signed webhooks. Covered by `TestWorkflowHandlerWebhookStartsExecutionWithRawPayload`, `TestWorkflowHandlerSignedWebhookStartsExecutionForValidSignature`, replay/expiry/signature rejection tests, and `TestRegisterWorkflowRoutesDispatchesSignedWebhookWithoutSession`. |
| Semantic trigger with keywords and embedding threshold. | Proven | Keyword semantic triggers and Chat send-message dispatcher wiring are proven. `EmbeddingSemanticTriggerMatcher` supports `semanticThreshold`, propagates user/org identity, and the default router/server workflow service now injects a Relay-backed matcher when Relay is enabled. `WORKFLOW_RELAY_BASE_URL` can override the local `/v1` Relay endpoint for deployments/tests. Covered by `TestServiceMatchSemanticTriggersReturnsPublishedKeywordMatches`, `TestServiceMatchSemanticTriggersUsesInjectedMatcherForThresholdTriggers`, `TestEmbeddingSemanticTriggerMatcherChoosesBestKeywordAndPropagatesIdentity`, `TestSendMessageTriggersSemanticWorkflowsAfterAssistantReply`, `TestNewConfiguredWorkflowServiceWiresRelaySemanticTriggerMatcher`, `TestLoadWorkflowSystemLimitConfig`, and HTTP route tests. |

## 2.2 Node Failure Handling

| Requirement | Status | Evidence |
| --- | --- | --- |
| Default `pause_on_failure` pauses for user decision. | Proven | `ApplyNodeFailure` and `Service.RecordNodeStatus` default to paused execution. Covered by `TestApplyNodeFailureDefaultsToPauseForUserDecision`, `TestServiceRecordNodeStatusAppliesFailurePolicies`, and paused failure decision HTTP tests. |
| `auto_retry` uses retries with exponential delays. | Proven | Runtime failure policy uses default `1s`, `3s`, `9s` delays and retrying node status. Covered by `TestApplyNodeFailureUsesExponentialDefaultRetryDelays`, `TestServiceRunExecutionUntilBlockedWaitsForAutoRetryDelay`, and `TestServiceRunExecutionUntilBlockedRunsDueAutoRetry`. |
| `skip_on_failure` marks node skipped and continues as partial success. | Proven | `RecordNodeStatus` seeds downstream nodes after skip and marks `partial_success`; `RunExecutionUntilBlocked` continues after skipped node. Covered by `TestApplyNodeFailureSkipsOptionalNodeAndContinues`, `TestServiceRecordNodeStatusAppliesFailurePolicies`, and `TestServiceRunExecutionUntilBlockedContinuesAfterSkipOnFailure`. |
| `failure_branch` routes error context to a branch. | Proven | `seedFailureBranchNode` merges `workflow.error` into branch input/context. Covered by `TestApplyNodeFailureBranchesToFailureNodeWithErrorContext`, `TestServiceRecordNodeStatusAppliesFailurePolicies`, and `TestServiceRunExecutionFailureBranchRunsWithErrorContext`. |
| User options: retry node, skip node, edit input retry, terminate workflow. | Proven | `ResolvePausedFailure` supports retry, continue, branch, and fail decisions with edited input. Covered by `TestServiceResolvePausedFailureDecisionRetriesSkipsAndTerminates` and `TestWorkflowHandlerWorkflowDecisionSupportsPausedFailureUserActions`. |
| Notify user on pause/failure. | Partial | The paused execution and node records expose reason/error to API/UI. `pause_on_failure` now emits a workflow failure-pause event through `WithFailurePauseNotificationSink`; the default HTTP/server wiring adapts that event into an in-app notification with workflow/execution/node metadata and an action URL. Covered by `TestServiceRecordNodeStatusNotifiesUserWhenFailurePausesExecution`, `TestNewConfiguredWorkflowServiceWiresFailurePauseNotification`, and `TestRouteSurfaceWiresConfiguredWorkflowSystemLimits`. Email/Webhook delivery for this exact workflow failure-pause event is still not proven. |

## 2.3 Concurrency Control

| Requirement | Status | Evidence |
| --- | --- | --- |
| Workflow-level configurable `max_concurrent_executions`, default 10. | Proven | `resourcePolicyForWorkflow` reads multiple key styles; `StartExecution` queues or rejects by workflow limit. Covered by `TestServiceStartExecutionRejectsWhenWorkflowConcurrencyLimitIsReached`. |
| Organization-level shared concurrent workflow limit, default 50. | Proven | `WithOrgMaxConcurrentWorkflows` and `CountRunningExecutionsForOrganization`; execution queues when org limit is reached. Covered by org limit tests around start/promotion behavior. |
| Trigger-aware smart defaults: conversation high, schedule serial, webhook medium. | Proven | `concurrencyPolicyForTrigger` sets conversation 50, schedule 1, webhook 10 unless workflow override exists. Covered by `TestServiceStartExecutionQueuesScheduleTriggersSeriallyByDefault` and `TestServicePromoteQueuedExecutionsKeepsScheduleTriggersSerial`. |
| System-level max concurrent workflows and global workflow executions/minute. | Proven | `SystemWorkflowLimits` and `WithSystemWorkflowLimits` add service-level guardrails for `MaxConcurrentWorkflows` and `MaxExecutionsPerMinute`. `StartExecution` rejects new executions before persistence when the configured global running-execution count or in-process one-minute sliding window is exceeded, while existing organization/workflow limits remain intact. `WORKFLOW_SYSTEM_MAX_CONCURRENT` and `WORKFLOW_GLOBAL_MAX_EXECUTIONS_PER_MINUTE` load into config, and the default router/server workflow service path passes those values through `newConfiguredWorkflowService`. Covered by `TestServiceStartExecutionRejectsWhenSystemConcurrencyLimitIsReached`, `TestServiceStartExecutionRejectsWhenGlobalExecutionsPerMinuteLimitIsReached`, `TestLoadWorkflowSystemLimitConfig`, and `TestRouteSurfaceWiresConfiguredWorkflowSystemLimits`. |

## 2.4 Resource Limits

| Requirement | Status | Evidence |
| --- | --- | --- |
| Execution timeout defaults to one hour and can be configured. | Proven | `CheckResourceLimits` marks status `timeout`; covered by `TestServiceCheckResourceLimitsTimesOutLongRunningExecution`. |
| Token budget pauses workflow and surfaces a user-visible reason. | Proven | `CheckResourceLimits` pauses execution and now records a failed `workflow_resource_guard` node with `token_budget_exceeded`, `maxTokensBudget`, and `totalTokens`. Covered by `TestServiceCheckResourceLimitsPausesWhenTokenBudgetIsExceeded`. |
| Node execution count prevents infinite loops and marks `max_iterations`. | Proven | `RunExecutionUntilBlocked` and `CheckResourceLimits` enforce `max_node_executions`; covered by `TestServiceCheckResourceLimitsStopsWhenNodeExecutionLimitIsExceeded` and `TestServiceRunExecutionUntilBlockedAllowsExecutionAtNodeLimit`. |
| Organization-level concurrency before start. | Proven | See 2.3 org-level concurrency evidence. |

## 2.5 Variable Scope

| Requirement | Status | Evidence |
| --- | --- | --- |
| Workflow input/global variables and node outputs are available to downstream nodes. | Proven | Runtime interpolation supports `{{input.*}}`, `{{workflow.name}}`, and `{{nodes.node_id.output.*}}`. Covered by `TestServiceRunReadyNodeInterpolatesWorkflowInputAndPriorNodeOutput`. |
| System variables include execution id, start time, workflow name, user id, and org id. | Proven | Covered by `TestServiceRunReadyNodeInterpolatesSystemExecutionAndOrganizationVariables` and `TestServiceRunReadyNodeInterpolatesUserVariableFromExecutionContextAndTriggerPayload`. |
| Variable inspection/debug snapshot. | Proven | `BuildExecutionDebugSnapshot` returns variable snapshot, trace, outputs, performance, and logs; HTTP/UI API exposes debug snapshot. Covered by workflow debug snapshot route tests and Workflows API types. |
| Node-local variable lifetime. | Proven | Node definitions can declare `variables`, and `RunReadyNode` resolves those values into the current node-only namespace `node.{node_id}.{var_name}` before interpolating the node input. Other nodes cannot read a previous node's `node.*` locals and must use `nodes.{node_id}.output.*` for cross-node data. Covered by `TestServiceRunReadyNodeInterpolatesCurrentNodeLocalVariablesOnly`. |

## 2.6 Version Management

| Requirement | Status | Evidence |
| --- | --- | --- |
| Saving updates creates version history. | Proven | `UpdateWorkflow` increments version and stores history; `TestServiceUpdateWorkflowCreatesVersionHistory` and SQL store version tests. |
| Running executions bind to published version and snapshot, not draft edits. | Proven | `StartExecution` uses latest published version and stores `WorkflowVersion`/`WorkflowSnapshot`; `TestServiceStartExecutionBindsLatestPublishedVersion` and store tests. |
| Rollback creates a new version from historical definition. | Proven | `RollbackWorkflow` creates a new version; covered by `TestServiceRollbackWorkflowCreatesNewVersionFromHistory`, `TestWorkflowHandlerRollbackWorkflowPassesVersion`, and route tests. |
| Branch from historical version for experiments. | Proven | `CreateWorkflowBranch` copies a version as draft experiment metadata; covered by `TestServiceCreateWorkflowBranchCopiesVersionAsDraftExperiment`, handler tests, and route tests. |
| Branch merge back to mainline or publish as new workflow. | Proven | `PublishWorkflowBranch` publishes a draft branch definition as a new published workflow without experiment metadata, and `MergeWorkflowBranch` applies branch definition/variables back to the source workflow as a new published mainline version. HTTP routes expose `POST /api/v1/workflows/{source}/branches/{branch}/publish` and `POST /api/v1/workflows/{source}/branches/{branch}/merge`, and the web API client calls both action endpoints. Covered by `TestServicePublishWorkflowBranchAsNewWorkflow`, `TestServiceMergeWorkflowBranchBackToMainline`, `TestWorkflowHandlerPublishWorkflowBranchPassesRequest`, `TestWorkflowHandlerMergeWorkflowBranchPassesRequest`, `TestRegisterWorkflowRoutesDispatchesBranchPublishAndMerge`, and `workflowsApi.test.ts`. |

## Fusion Design 3.2 and Frontend/API Surface

| Requirement | Status | Evidence |
| --- | --- | --- |
| Workflow CRUD, execute, test-node, executions, pause, resume API. | Proven | API routes in `src/server/internal/http/routes_workflow.go`; tests include `TestRegisterWorkflowRoutesDispatchesWorkflowCrudAndTestNode`, execution action tests, and handler tests. |
| React Flow visual editor and debug panel. | Proven for core UI | `WorkflowsPage.tsx` uses `@xyflow/react`, node palette, trigger/resource/failure policy forms, test node, debug snapshot, version/rollback/branch controls. `WorkflowsPage.test.tsx` and `workflowsApi.test.ts` cover key UI/API behavior. |
| 20+ node types. | Partial | Runtime supports many concrete node categories including start/manual/agent/LLM/knowledge/condition/loop/code/HTTP/tool/database/RPA/user_input, with executor tests. The audit did not prove 20+ distinct draggable node components or callable executors. |
| Complete end-to-end production success rate target >99%. | Unverified | Metrics exist for workflow execution and node errors, but success-rate SLO is not proven by load/production evidence. |

## Current Conclusion

The repository-owned Workflow engine now proves most Functional Logic 2.1-2.6 behavior: trigger modes, default schedule synchronization wiring, Chat-driven conversation/semantic dispatcher wiring, Relay-backed semantic-threshold matcher wiring, DAG execution, failure strategies, in-app failure-pause notification, concurrency queues/rejects, resource timeout/token/node limits, variable interpolation/debug snapshots, version history, rollback, and branch creation.

The matrix row remains `Partial`, not `Proven`, because these requirements still need work or stronger evidence:

1. Prove email/Webhook delivery for workflow failure-pause notifications, beyond the covered in-app path.
2. Prove 20+ node types and production-grade frontend drag/drop workflows end to end.
