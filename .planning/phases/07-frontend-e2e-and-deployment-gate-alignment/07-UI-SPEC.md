---
phase: 07
slug: frontend-e2e-and-deployment-gate-alignment
status: approved
shadcn_initialized: true
preset: radix-maia
created: 2026-05-17
reviewed_at: 2026-05-17
---

# Phase 07 - UI Design Contract

Visual and interaction contract for Phase 07 planning. This phase does not create a new visual direction; it aligns the existing frontend, E2E coverage, CI, and deployment gates with the Phase 06 backend surface.

## Design System

| Property | Value |
|----------|-------|
| Tool | shadcn |
| Preset | radix-maia |
| Component library | Radix UI primitives via shadcn wrappers in `src/web/src/components/ui/` |
| Icon library | Remix Icon (`@remixicon/react`) |
| Font | Figtree Variable for headings, Geist Mono Variable for app/body text |

## Visual Hierarchy and Interaction Contract

| Surface | Focal point | Required interaction behavior |
|---------|-------------|-------------------------------|
| Workspace Knowledge | Knowledge base or document title | Primary actions are `Create knowledge base`, `Save knowledge base`, `Create document`, `Save document`, and `Search knowledge`; destructive document/base actions stay secondary and explicit. |
| Workspace SOLO | Task goal editor | Primary action is `Start solo run`; budget, execution mode, authorization scope, knowledge source, allow-list, and deny-list controls remain visible before execution. |
| Marketplace | Agent or MCP server result card | Search/filter controls remain above cards; install/publish actions use specific labels such as `Install Agent`, `Publish Agent`, and `Install Server`. |
| Admin pages | Page heading and data table | Admin navigation must keep current routes visible and use dense, scannable tables rather than marketing-style cards. |

## Spacing Scale

Declared values must remain multiples of 4.

| Token | Value | Usage |
|-------|-------|-------|
| xs | 4px | Icon gaps and compact inline spacing |
| sm | 8px | Button/input internal spacing and compact stacks |
| md | 16px | Default form and card spacing |
| lg | 24px | Page panel and section padding |
| xl | 32px | Major layout columns and page-level gaps |
| 2xl | 48px | Large empty-state and route group separation |
| 3xl | 64px | Top-level marketing-only spacing; avoid in operational workspace views |

Exceptions: none for Phase 07.

## Typography

| Role | Size | Weight | Line Height |
|------|------|--------|-------------|
| Label | 14px | 500 | 1.4 |
| Body | 16px | 400 | 1.5 |
| Heading | 20px | 600 | 1.3 |
| Display | 28px | 600 | 1.2 |

## Color

| Role | Value | Usage |
|------|-------|-------|
| Dominant (60%) | `var(--background)` / `oklch(1 0 0)` light, `oklch(0.147 0.004 49.3)` dark | App background and page body |
| Secondary (30%) | `var(--card)`, `var(--muted)`, `var(--sidebar)` | Cards, tables, nav/sidebar, subdued panels |
| Accent (10%) | `var(--primary)` / `oklch(0.488 0.243 264.376)` light, `oklch(0.424 0.199 265.638)` dark | Primary CTA, active nav item, selected tab, focus ring |
| Destructive | `var(--destructive)` / `oklch(0.577 0.245 27.325)` light, `oklch(0.704 0.191 22.216)` dark | Delete, disable, cancel-run, and reject actions only |

Accent reserved for: primary CTA, active nav item, selected tab, focus ring. It is not used for every interactive element.

## Copywriting Contract

| Element | Copy |
|---------|------|
| Primary CTA | `Start solo run`, `Create knowledge base`, `Search knowledge`, `Publish Agent`, `Install Agent`, `Run deployment validation` |
| Empty state heading | `No knowledge bases yet`, `No documents yet`, `No running tasks`, `No marketplace matches` |
| Empty state body | `Create one to start collecting workspace context.`, `Add one to seed this knowledge base.`, `Start a SOLO task with a clear goal and selected knowledge sources.`, `Adjust the search or category filters.` |
| Error state | `Unable to load workspace data. Retry the request or check the backend session.` |
| Destructive confirmation | `Delete knowledge base`: confirm the selected base and documents will be removed. `Delete document`: confirm the named document will be removed. `Cancel task`: confirm the run will stop. |

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| shadcn official | button, card, checkbox, command, dialog, dropdown-menu, input, scroll-area, select, separator, sheet, skeleton, table, tabs, textarea, toggle, tooltip | not required |
| third-party registries | none | not applicable |

## Phase 07 Implementation Constraints

- Preserve the shadcn/Radix component layer in `src/web/src/components/ui/`; do not add a second UI framework.
- Preserve existing route copy unless a backend contract mismatch forces a rename.
- Do not introduce generated reports, Playwright traces, `dist/`, `.tmp/`, or cache directories into the staged frontend/deployment commit groups.
- Keep Phase 07 UI changes limited to frontend/API alignment and E2E operability. Contract docs remain Phase 08.

## Checker Sign-Off

- [x] Dimension 1 Copywriting: PASS
- [x] Dimension 2 Visuals: PASS
- [x] Dimension 3 Color: PASS
- [x] Dimension 4 Typography: PASS
- [x] Dimension 5 Spacing: PASS
- [x] Dimension 6 Registry Safety: PASS

**Approval:** approved 2026-05-17
