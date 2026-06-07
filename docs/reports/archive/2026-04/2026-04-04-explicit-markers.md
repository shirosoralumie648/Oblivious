# Historical Material Notice

> 本文件属于历史阶段材料，不再作为当前现状、优先级判断或里程碑规划依据。
> 当前唯一执行基线请改看：`CURRENT_STATUS.md`、`ROADMAP.md`、`docs/architecture/current-system-contracts.md`

# Oblivious 显式 TODO / FIXME / TBD 标记清单

日期：2026-04-04

## 1. 统计口径

本清单基于注释/文档型 marker 过滤，不计入以下非注释误命中：

- `context.TODO()`
- 测试字符串字面量中的 `TODO`
- 业务日志文本中的 `TODO`

统计结果：

- 主线 `src/server` / `src/web` / `docs`：0
- `new-api`：125
  - `TODO` 124
  - `FIXME` 1
- `lobehub`：116
  - `TODO` 112
  - `FIXME` 3
  - `TBD` 1

## 2. 主线仓结果

当前主线仓未发现显式注释 marker。

## 3. `new-api` 标记清单

```text
new-api/electron/README.md:17:TODO
new-api/relay/channel/mokaai/adaptor.go:23:	//TODO implement me
new-api/relay/channel/mokaai/adaptor.go:28:	//TODO implement me
new-api/relay/channel/mokaai/adaptor.go:34:	//TODO implement me
new-api/relay/channel/mokaai/adaptor.go:39:	//TODO implement me
new-api/relay/channel/mokaai/adaptor.go:44:	//TODO implement me
new-api/relay/channel/mokaai/adaptor.go:86:	// TODO implement me
new-api/relay/channel/palm/adaptor.go:22:	//TODO implement me
new-api/relay/channel/palm/adaptor.go:27:	//TODO implement me
new-api/relay/channel/palm/adaptor.go:33:	//TODO implement me
new-api/relay/channel/palm/adaptor.go:38:	//TODO implement me
new-api/relay/channel/palm/adaptor.go:67:	//TODO implement me
new-api/relay/channel/palm/adaptor.go:72:	// TODO implement me
new-api/relay/channel/vertex/adaptor.go:105:	//TODO implement me
new-api/relay/channel/vertex/adaptor.go:355:	//TODO implement me
new-api/relay/channel/vertex/adaptor.go:360:	// TODO implement me
new-api/relay/channel/gemini/adaptor.go:56:	//TODO implement me
new-api/relay/channel/gemini/adaptor.go:241:	// TODO implement me
new-api/relay/channel/claude/relay-claude.go:235:		// TODO: 临时处理
new-api/relay/channel/claude/relay-claude.go:434:					// FIXME
new-api/relay/channel/claude/adaptor.go:23:	//TODO implement me
new-api/relay/channel/claude/adaptor.go:32:	//TODO implement me
new-api/relay/channel/claude/adaptor.go:37:	//TODO implement me
new-api/relay/channel/claude/adaptor.go:106:	//TODO implement me
new-api/relay/channel/claude/adaptor.go:111:	// TODO implement me
new-api/relay/channel/baidu_v2/adaptor.go:24:	//TODO implement me
new-api/relay/channel/baidu_v2/adaptor.go:34:	//TODO implement me
new-api/relay/channel/baidu_v2/adaptor.go:39:	//TODO implement me
new-api/relay/channel/baidu_v2/adaptor.go:105:	//TODO implement me
new-api/relay/channel/baidu_v2/adaptor.go:110:	// TODO implement me
new-api/service/billing_session.go:178:		// TODO: model 层应定义哨兵错误（如 ErrNoActiveSubscription），用 errors.Is 替代字符串匹配
new-api/relay/channel/jina/adaptor.go:24:	//TODO implement me
new-api/relay/channel/jina/adaptor.go:29:	//TODO implement me
new-api/relay/channel/jina/adaptor.go:35:	//TODO implement me
new-api/relay/channel/jina/adaptor.go:40:	//TODO implement me
new-api/relay/channel/jina/adaptor.go:67:	// TODO implement me
new-api/relay/channel/baidu/adaptor.go:23:	//TODO implement me
new-api/relay/channel/baidu/adaptor.go:28:	//TODO implement me
new-api/relay/channel/baidu/adaptor.go:34:	//TODO implement me
new-api/relay/channel/baidu/adaptor.go:39:	//TODO implement me
new-api/relay/channel/baidu/adaptor.go:142:	// TODO implement me
new-api/relay/channel/coze/adaptor.go:23:	//TODO implement me
new-api/service/sensitive.go:20:				// TODO: check image url
new-api/relay/channel/coze/relay-coze.go:31:				// TODO: support more content type
new-api/relay/channel/deepseek/adaptor.go:24:	//TODO implement me
new-api/relay/channel/deepseek/adaptor.go:34:	//TODO implement me
new-api/relay/channel/deepseek/adaptor.go:39:	//TODO implement me
new-api/relay/channel/deepseek/adaptor.go:82:	//TODO implement me
new-api/relay/channel/deepseek/adaptor.go:87:	// TODO implement me
new-api/model/main.go:202:			//_, _ = sqlDB.Exec("ALTER TABLE channels MODIFY model_mapping TEXT;") // TODO: delete this line when most users have upgraded
new-api/relay/channel/xunfei/adaptor.go:22:	//TODO implement me
new-api/relay/channel/xunfei/adaptor.go:27:	//TODO implement me
new-api/relay/channel/xunfei/adaptor.go:33:	//TODO implement me
new-api/relay/channel/xunfei/adaptor.go:38:	//TODO implement me
new-api/relay/channel/xunfei/adaptor.go:67:	//TODO implement me
new-api/relay/channel/xunfei/adaptor.go:72:	// TODO implement me
new-api/model/utils.go:75:		// TODO: maybe we can combine updates with same key?
new-api/relay/channel/mistral/adaptor.go:21:	//TODO implement me
new-api/relay/channel/mistral/adaptor.go:26:	//TODO implement me
new-api/relay/channel/mistral/adaptor.go:32:	//TODO implement me
new-api/relay/channel/mistral/adaptor.go:37:	//TODO implement me
new-api/relay/channel/mistral/adaptor.go:66:	//TODO implement me
new-api/relay/channel/mistral/adaptor.go:71:	// TODO implement me
new-api/relay/channel/moonshot/adaptor.go:25:	//TODO implement me
new-api/relay/channel/moonshot/adaptor.go:35:	//TODO implement me
new-api/relay/channel/moonshot/adaptor.go:86:	// TODO implement me
new-api/relay/channel/ali/adaptor.go:50:	//TODO implement me
new-api/relay/channel/ali/adaptor.go:210:	//TODO implement me
new-api/relay/channel/tencent/adaptor.go:30:	//TODO implement me
new-api/relay/channel/tencent/adaptor.go:35:	//TODO implement me
new-api/relay/channel/tencent/adaptor.go:41:	//TODO implement me
new-api/relay/channel/tencent/adaptor.go:46:	//TODO implement me
new-api/relay/channel/tencent/adaptor.go:91:	//TODO implement me
new-api/relay/channel/tencent/adaptor.go:96:	// TODO implement me
new-api/relay/channel/cloudflare/adaptor.go:24:	//TODO implement me
new-api/relay/channel/cloudflare/adaptor.go:29:	//TODO implement me
new-api/relay/channel/cloudflare/adaptor.go:102:	//TODO implement me
new-api/relay/channel/zhipu_4v/adaptor.go:26:	//TODO implement me
new-api/relay/channel/zhipu_4v/adaptor.go:35:	//TODO implement me
new-api/relay/channel/zhipu_4v/adaptor.go:102:	// TODO implement me
new-api/relay/channel/siliconflow/adaptor.go:25:	//TODO implement me
new-api/relay/channel/siliconflow/adaptor.go:96:	// TODO implement me
new-api/relay/channel/dify/adaptor.go:29:	//TODO implement me
new-api/relay/channel/dify/adaptor.go:34:	//TODO implement me
new-api/relay/channel/dify/adaptor.go:40:	//TODO implement me
new-api/relay/channel/dify/adaptor.go:45:	//TODO implement me
new-api/relay/channel/dify/adaptor.go:93:	//TODO implement me
new-api/relay/channel/dify/adaptor.go:98:	// TODO implement me
new-api/relay/channel/task/gemini/image.go:56:// TODO: support downloading HTTP URL images and converting to base64
new-api/relay/channel/task/gemini/dto.go:14:	// TODO: support referenceImages (style/asset references, up to 3 images)
new-api/relay/channel/task/gemini/dto.go:15:	// TODO: support lastFrame (first+last frame interpolation, Veo 3.1)
new-api/relay/channel/perplexity/adaptor.go:24:	//TODO implement me
new-api/relay/channel/perplexity/adaptor.go:34:	//TODO implement me
new-api/relay/channel/perplexity/adaptor.go:39:	//TODO implement me
new-api/relay/channel/perplexity/adaptor.go:74:	//TODO implement me
new-api/relay/channel/zhipu/adaptor.go:22:	//TODO implement me
new-api/relay/channel/zhipu/adaptor.go:27:	//TODO implement me
new-api/relay/channel/zhipu/adaptor.go:33:	//TODO implement me
new-api/relay/channel/zhipu/adaptor.go:38:	//TODO implement me
new-api/relay/channel/zhipu/adaptor.go:75:	//TODO implement me
new-api/relay/channel/zhipu/adaptor.go:84:	// TODO implement me
new-api/relay/channel/aws/adaptor.go:37:	//TODO implement me
new-api/relay/channel/aws/adaptor.go:79:	//TODO implement me
new-api/relay/channel/aws/adaptor.go:84:	//TODO implement me
new-api/relay/channel/aws/adaptor.go:139:	//TODO implement me
new-api/relay/channel/aws/adaptor.go:144:	// TODO implement me
new-api/relay/channel/cohere/adaptor.go:22:	//TODO implement me
new-api/relay/channel/cohere/adaptor.go:27:	//TODO implement me
new-api/relay/channel/cohere/adaptor.go:33:	//TODO implement me
new-api/relay/channel/cohere/adaptor.go:38:	//TODO implement me
new-api/relay/channel/cohere/adaptor.go:64:	// TODO implement me
new-api/relay/channel/cohere/adaptor.go:77:	//TODO implement me
new-api/relay/channel/cohere/adaptor.go:86:			usage, err = cohereStreamHandler(c, info, resp) // TODO: fix this
new-api/relay/channel/xai/adaptor.go:25:	//TODO implement me
new-api/relay/channel/xai/adaptor.go:30:	//TODO implement me
new-api/relay/channel/volcengine/adaptor.go:36:	//TODO implement me
new-api/relay/channel/jimeng/adaptor.go:24:	//TODO implement me
new-api/relay/claude_handler.go:77:			// TODO: 临时处理
new-api/middleware/distributor.go:387:	// TODO: api_version统一
new-api/controller/channel-billing.go:466:		// TODO: support Azure
new-api/controller/channel-billing.go:485:	// TODO: make it async
new-api/controller/relay.go:245:	Subprotocols: []string{"realtime"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol TODO add other protocol
new-api/setting/ratio_setting/model_ratio.go:22:// TODO: when a new api is enabled, check the pricing here
new-api/dto/gemini.go:169:	// TODO Conflict with thinkingbudget.
new-api/common/gin.go:126:		// TODO: someday non json request have variant model, we will need to implementation this
```

## 4. `lobehub` 标记清单

```text
lobehub/vitest.config.mts:12:  // TODO: after refactor the errorResponse, we can remove it
lobehub/packages/python-interpreter/src/worker.ts:75:      // TODO: Consider using WORKERFS here to reduce one copy operation
lobehub/packages/builtin-tool-group-management/src/executor.ts:234:    // TODO: Implement conversation summarization
lobehub/packages/builtin-tool-group-management/src/executor.ts:251:    // TODO: Implement workflow creation
lobehub/packages/builtin-tool-group-management/src/executor.ts:259:    // TODO: Implement voting mechanism
lobehub/packages/model-bank/src/aiModels/bedrock.ts:152:    // TODO: Not support for now
lobehub/packages/model-bank/src/aiModels/lobehub/chat/xiaomimimo.ts:38:        // TODO: restore actual pricing when promotion ends
lobehub/packages/model-bank/src/aiModels/lobehub/chat/xiaomimimo.ts:78:        // TODO: restore actual pricing when promotion ends
lobehub/packages/model-bank/src/aiModels/lobehub/chat/xiaomimimo.ts:108:        // TODO: restore actual pricing when promotion ends
lobehub/packages/types/src/message/common/metadata.ts:42:  // TODO: make all following fields required
lobehub/packages/model-bank/src/aiModels/volcengine.ts:1200:    // TODO: AIImageModelCard does not support config.deploymentName
lobehub/packages/model-bank/src/aiModels/volcengine.ts:1227:    // TODO: AIImageModelCard does not support config.deploymentName
lobehub/packages/builtin-tool-web-browsing/src/client/Render/PageContent/index.tsx:36:            // TODO: Remove this in v2 as it's deprecated
lobehub/packages/types/src/index.ts:42:// FIXME: I think we need a refactor for the "openai" types
lobehub/packages/types/src/files/upload.ts:71:  // TODO: Need be required
lobehub/packages/types/src/userMemory/list.ts:22:// TODO: Extend to other source types later, e.g. Notion, Obsidian, YuQue
lobehub/packages/model-runtime/src/core/usageConverters/utils/computeChatCost.ts:112:  // TODO: Support this when ModelTokensUsage includes this data
lobehub/packages/model-runtime/src/core/usageConverters/utils/computeChatCost.ts:147: * TODO: Some providers do use tiered calculation, such as Zhipu
lobehub/packages/model-runtime/src/core/usageConverters/utils/computeImageCost.ts:93:      // TODO: Implement tiered pricing when needed
lobehub/packages/types/src/tool/plugin.ts:32:   * TODO: Temporary solution, needs major refactoring in the future
lobehub/packages/model-runtime/src/core/streams/cloudflare.ts:12:    if (!res) return; // TODO: Add test; Handle tool_call parameter.
lobehub/packages/model-runtime/src/core/streams/spark.ts:162:  // TODO: preserve for RFC 097
lobehub/packages/types/src/tool/index.ts:14:  // TODO: remove type and then make it required
lobehub/packages/types/src/exportConfig.ts:6:// ---------- TODO: this file need to be deleted in V2 ---------- //
lobehub/packages/model-runtime/src/core/streams/qwen.ts:148:  // TODO: preserve for RFC 097
lobehub/apps/cli/src/commands/migrate/openclaw.ts:211:    // Filter out placeholder text like （待定）, _(待定)_, (TBD), N/A, etc.
lobehub/packages/model-runtime/src/core/openaiCompatibleFactory/index.ts:676:            // TODO: should refactor after remove v1 user/modelList code
lobehub/packages/model-runtime/src/utils/getModelPricing.ts:7: * TODO: Add a fallback provider priority list. When no provider is specified,
lobehub/packages/model-runtime/vitest.config.mts:10:      // TODO: 目前仍然残留 ModelRuntime.test.ts 中的部分测试依赖了主项目的内容，后续需要拆分测试
lobehub/packages/model-runtime/src/const/models.ts:8:// TODO: temporary implementation, needs to be refactored into model card display configuration
lobehub/packages/memory-user-memory/src/extractors/base.ts:94:      // TODO: additional messages typing issue
lobehub/packages/memory-user-memory/src/schemas/identity.ts:102:          // TODO: OpenAI requires `required` fields to be always present, while enum fields cannot be null
lobehub/packages/memory-user-memory/src/schemas/identity.ts:113:          // TODO: OpenAI requires `required` fields to be always present, while enum fields cannot be null
lobehub/packages/memory-user-memory/src/prompts/persona.ts:1:// TODO(@nekomeowww): introduce profile when multi-persona is enabled.
lobehub/packages/chat-adapter-qq/src/adapter.ts:237:    // TODO: Implement message recall if QQ API supports it
lobehub/packages/agent-runtime/src/groupOrchestration/types.ts:346:// TODO: Remove these after migration is complete
lobehub/packages/database/src/schemas/userMemories/persona.ts:7:// TODO(@nekomeowww): add a comment/annotation layer for personas.
lobehub/packages/database/src/repositories/dataImporter/deprecated/index.ts:318:          // TODO: Need to handle TTS and image insertion in the future (currently difficult to handle due to file-related parts)
lobehub/packages/prompts/src/prompts/chatMessages/index.ts:40: * TODO: Use context engineering to filter messages
lobehub/packages/database/src/repositories/aiInfra/index.ts:465:      // TODO: when model-bank is a separate module, we will try import from model-bank/[prividerId] again
lobehub/src/app/(backend)/api/webhooks/video/[provider]/route.ts:18:// TODO: temporarily disabled until notification UI is polished
lobehub/src/app/(backend)/api/webhooks/video/[provider]/route.ts:206:    // TODO: temporarily disabled until notification UI is polished
lobehub/src/app/(backend)/api/workflows/memory-user-memory/pipelines/chat-topic/process-user-topics/route.ts:100:      // TODO: follow the new pattern of process-topic
lobehub/packages/database/src/models/session.ts:176:  // TODO: In the future, once Inbox ID is stored in the database, we can directly use the _rank method
lobehub/packages/database/src/models/session.ts:608:    // TODO: Need a better implementation in the future, currently only taking the first one
lobehub/packages/database/src/models/message.ts:1269:          // TODO: remove this when the client is updated
lobehub/packages/database/src/models/message.ts:1323:      // TODO: need a better way to handle this
lobehub/packages/database/src/models/userMemory/model.ts:1425:    // TODO(@nekomeowww): activity
lobehub/src/libs/traces/event.ts:121:      // TODO: add tag when supported
lobehub/src/services/_header.ts:6: * TODO: Need to be removed after tts refactor
lobehub/src/services/session/index.ts:62:  // TODO: Need to be fixed
lobehub/src/libs/trusted-client/index.ts:37:      // TODO: remove '' when sdk update
lobehub/src/libs/better-auth/define-config.ts:213:              // TODO: if add phone plugin, we should fill phone here
lobehub/src/services/chat/helper.ts:15: * TODO: we need to update this function to auto find deploymentName with provider setting config
lobehub/src/store/home/slices/homeInput/action.ts:161:    // TODO: Implement DeepResearch mode
lobehub/src/services/chat/mecha/contextEngineering.ts:328:          completed: false, // TODO: Add completed field to document if needed
lobehub/src/store/task/slices/config/action.ts:107:  // TODO [LOBE-6587]: 定时任务（cron 模式）
lobehub/src/store/session/slices/session/reducers.ts:30:        // TODO: Migrate Date type in the future to remove this ignore
lobehub/src/utils/errorResponse.ts:25:    // TODO: Need to refactor to Invalid OpenAI API Key
lobehub/src/store/session/slices/homeInput/action.ts:83:    // TODO: Implement DeepResearch mode
lobehub/src/store/task/selectors/detailSelectors.ts:27:// TODO [LOBE-6634]: 等后端 getTaskDetail 返回 model/provider 后，改为读 detail.model / detail.provider
lobehub/src/store/chat/slices/message/actions/query.ts:38:    // TODO: Support threadId refresh when needed
lobehub/src/features/ResourceManager/components/Header/AddButton.tsx:52:  // TODO: Migrate Notion import to use createResource
lobehub/src/features/Conversation/Error/OllamaSetupGuide/Desktop.tsx:10:// TODO: Optimize the Ollama setup flow - in isDesktop mode, end-to-end detection can be done directly
lobehub/src/store/chat/slices/portal/initialState.ts:43:  // TODO: Remove after Phase 3 migration complete
lobehub/src/store/chat/slices/plugin/action.test.ts:720:      // TODO: 需要验证 updateMessage 是否被调用
lobehub/src/store/chat/slices/builtinTool/actions/interpreter.ts:68:      // TODO: should only download files used by the AI
lobehub/src/features/ResourceManager/components/Explorer/index.tsx:63:  // TODO: Eventually update all consumers to use ResourceItem directly
lobehub/src/hooks/useTokenCount.test.ts:9:  // TODO: need to be fixed in the future
lobehub/src/store/chat/slices/aiChat/actions/streamingExecutor.ts:463:      // TODO: Support dedicated compression model from chatConfig.compressionModelId
lobehub/src/store/chat/slices/topic/initialState.ts:23:  // TODO: need to add the null to the type
lobehub/src/store/chat/slices/topic/reducer.ts:53:            // TODO: updatedAt type needs to be changed to Date later
lobehub/src/store/chat/slices/topic/action.test.ts:1003:      // TODO: need to test with fetchPresetTaskResult
lobehub/src/store/chat/agents/createAgentExecutors.ts:242:        // TODO: Maybe this should be implemented with an init method in the future
lobehub/src/routes/(main)/home/_layout/Body/Agent/List/AgentItem/index.tsx:97:    group: undefined, // TODO: pass group from parent if needed
lobehub/src/features/Conversation/Messages/Task/components/MessageContent.tsx:26:    // TODO: Need to implement isIntentUnderstanding selector in ConversationStore if needed
lobehub/src/routes/(main)/settings/hooks/useCategory.tsx:104:      // TODO: temporarily disabled until notification UI is polished
lobehub/src/server/routers/mobile/topic.ts:94:  // TODO: this procedure should be used with authedProcedure
lobehub/src/server/routers/async/image.ts:11:// TODO: temporarily disabled until notification UI is polished
lobehub/src/server/routers/async/image.ts:151:  // FIXME: 401 errors should be handled in agentRuntime for better practice
lobehub/src/server/routers/async/image.ts:363:          // TODO: temporarily disabled until notification UI is polished
lobehub/src/server/services/discover/index.ts:2052:      // TODO: SDK method not yet available, using fallback
lobehub/src/server/services/discover/index.ts:2080:      // TODO: SDK method not yet available, using fallback
lobehub/src/server/services/discover/index.ts:2118:      // TODO: SDK method not yet available
lobehub/src/server/services/discover/index.ts:2127:      // TODO: SDK method not yet available
lobehub/src/routes/(main)/group/profile/features/Header/GroupPublishButton/useMarketGroupPublish.ts:129:        // TODO: Construct proper A2A URL for the agent
lobehub/src/routes/(main)/group/profile/features/Header/GroupPublishButton/useMarketGroupPublish.ts:139:        category: 'productivity', // TODO: Allow user to select category
lobehub/src/routes/(main)/group/profile/features/Header/GroupPublishButton/useMarketGroupPublish.ts:166:        visibility: 'public', // TODO: Allow user to select visibility
lobehub/src/server/services/mcp/index.ts:386:      // TODO: temporary
lobehub/src/server/services/mcp/index.ts:426:      // TODO: temporary
lobehub/src/routes/(main)/group/profile/features/GroupProfile/GroupStatusTag.tsx:32:        // TODO: Use getAgentGroupDetail when available
lobehub/src/server/services/queue/impls/qstash.ts:96:      // TODO: Implement cancellation logic, cancellation list can be stored via Redis
lobehub/src/server/services/memory/userMemory/extract.ts:544:      // TODO: we might need to think about how to deal with non-string contents
lobehub/src/server/services/memory/userMemory/extract.ts:1060:    // TODO: make topK configurable
lobehub/src/server/services/memory/userMemory/extract.ts:1424:            // TODO: make topK configurable
lobehub/src/server/services/memory/userMemory/extract.ts:2018:    // TODO: implement a better cache eviction strategy
lobehub/src/server/services/memory/userMemory/extract.ts:2019:    // TODO: make cache size configurable
lobehub/src/server/services/agentRuntime/AgentRuntimeService.ts:1638:      // TODO: implement approveToolCall logic
lobehub/src/server/services/agentRuntime/AgentRuntimeService.ts:1641:      // TODO: implement rejectToolCall logic
lobehub/src/server/services/agentRuntime/AgentRuntimeService.ts:1644:      // TODO: implement processHumanInput logic
lobehub/src/server/routers/lambda/market/agentGroup.ts:187:                // TODO: Add author info from group detail
lobehub/src/server/services/memory/userMemory/persona/service.ts:145:    // TODO(@nekomeowww): @arvinxx kindly take some time to review this policy
lobehub/src/routes/(main)/community/(detail)/features/MakedownRender.tsx:42:          // FIXME ignore experimental blob image prop passing
lobehub/src/routes/(main)/community/(detail)/group_agent/features/Header.tsx:66:  // TODO: Use 'group_agent' type when social service supports it
lobehub/src/server/routers/lambda/plugin.ts:68:  // TODO: In the future, this method also needs to use authedProcedure
lobehub/src/server/routers/lambda/user.ts:306:      // TODO: better to add a validation
lobehub/src/server/services/aiAgent/index.ts:738:            name: userModel.providerId, // TODO: Map to friendly provider name
lobehub/src/server/routers/lambda/chunk.ts:286:        // TODO: need to rerank the chunks
lobehub/src/server/services/taskScheduler/impls/index.ts:17:    // TODO: QStash implementation
lobehub/src/server/routers/lambda/__tests__/integration/message.integration.test.ts:192:      // TODO: This validation is not currently enforced in the code
lobehub/src/server/routers/lambda/aiModel.ts:55:        // TODO: Complete validation schema
lobehub/src/server/services/bot/platforms/qq/schema.ts:69:      // TODO: DM schema - not implemented yet
lobehub/src/server/services/bot/platforms/feishu/definitions/schema.ts:98:      // TODO: DM schema - not implemented yet
lobehub/src/server/services/bot/platforms/telegram/schema.ts:77:      // TODO: DM schema - not implemented yet
lobehub/src/server/services/bot/platforms/slack/schema.ts:92:      // TODO: DM schema - not implemented yet
lobehub/src/server/services/bot/platforms/discord/schema.ts:92:      // TODO: DM schema - not implemented yet
```

## 5. 备注

- 这些 marker 主要来自嵌入的上游独立仓，不直接代表当前根主线的交付缺口。
- 如果后续决定把 `new-api` 或 `lobehub` 真正纳入当前主线，需要先做一次“是否纳入 backlog”的治理决策，再决定是否拆成当前仓库任务。

