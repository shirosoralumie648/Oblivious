1. **Analyze Type Errors**: The CI is failing because `AdminChannelsPage.tsx` accesses `installable`, `configurable`, and `runtimeReady` on `ChannelProviderInfo`, but these properties are not defined in `src/web/src/types/admin.ts`.
2. **Fix `src/web/src/types/admin.ts`**: Update `ChannelProviderInfo` to include these missing properties as optional booleans (since they are accessed with `??`).
3. **Verify Build**: Run `cd src/web && pnpm build` to ensure there are no TypeScript errors.
4. **Pre-commit and Submit**.
