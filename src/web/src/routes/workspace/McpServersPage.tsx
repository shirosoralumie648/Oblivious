import { McpServersPanel } from '../../features/mcp/McpServersPanel';

export function McpServersPage() {
  return (
    <section className="mx-auto max-w-6xl space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">Agent tools</p>
        <h1 className="font-heading text-3xl font-semibold text-[#181611]">MCP Servers & Tools</h1>
      </header>

      <McpServersPanel />
    </section>
  );
}
