import { Link } from 'react-router-dom';
import {
  RiArrowRightLine,
  RiBillLine,
  RiDatabase2Line,
  RiFlowChart,
  RiRobot2Line,
  RiShieldCheckLine,
  RiStore2Line,
} from '@remixicon/react';

import { getGeneratedReleaseCapability } from '../../features/releaseProjection/releaseProjection';

const routeMap = [
  { path: '/chat', label: 'Relay chat', detail: 'Quota, model policy, drafts, and SOLO handoff.', capabilityId: 'chat.conversation_use' },
  { path: '/knowledge', label: 'Knowledge RAG', detail: 'Embedding retrieval with source citations.', capabilityId: 'knowledge.retrieval' },
  { path: '/solo', label: 'Agent runs', detail: 'Durable tool runs, approval gates, retry evidence.', capabilityId: 'agent.run' },
  { path: '/marketplace', label: 'Marketplace', detail: 'Browse, publish, install, review, settlement boundaries.', capabilityId: 'marketplace.commerce' },
  { path: '/admin/billing', label: 'Billing ops', detail: 'Sessions, invoices, refunds, payouts, webhook ledger.', capabilityId: 'billing.payment_lifecycle' },
];

const commandSurfaces = [
  { label: 'Tenant scope', value: 'locked', icon: <RiShieldCheckLine className="size-5" aria-hidden="true" /> },
  { label: 'Relay authority', value: 'metered', icon: <RiFlowChart className="size-5" aria-hidden="true" /> },
  { label: 'Billing ledger', value: 'audited', icon: <RiBillLine className="size-5" aria-hidden="true" /> },
  { label: 'Recovery proof', value: 'smoked', icon: <RiDatabase2Line className="size-5" aria-hidden="true" /> },
];

export function HomePage() {
  const committedRoutes = routeMap.filter((route) => {
    const generated = getGeneratedReleaseCapability(route.capabilityId);
    return generated?.disposition === 'committed' && generated.navigationDisposition === 'visible';
  });
  const authCommitted = getGeneratedReleaseCapability('identity.account_session')?.disposition === 'committed';
  const consoleCommitted = getGeneratedReleaseCapability('billing.ledger_lifecycle')?.disposition === 'committed';
  return (
    <main className="min-h-screen overflow-hidden bg-[#11100d] text-[#f7f4ea]" data-gsap-scope="marketing">
      <section className="relative min-h-[92vh] border-b border-white/10">
        <div
          className="absolute inset-0 opacity-70"
          aria-hidden="true"
          style={{
            backgroundImage:
              'linear-gradient(90deg, rgba(255,255,255,.07) 1px, transparent 1px), linear-gradient(rgba(255,255,255,.05) 1px, transparent 1px), linear-gradient(135deg, rgba(23,162,184,.18), transparent 34%, rgba(218,154,56,.22) 68%, transparent)',
            backgroundSize: '72px 72px, 72px 72px, 100% 100%',
          }}
        />
        <div className="relative mx-auto flex min-h-[92vh] max-w-7xl flex-col px-6 py-6 lg:px-8">
          <header className="flex items-center justify-between gap-4" data-gsap-item>
            <Link to="/" className="flex items-center gap-3 text-sm font-semibold">
              <span className="flex size-9 items-center justify-center rounded-lg border border-amber-300/30 bg-amber-300/10 text-amber-200">O</span>
              <span>Oblivious command plane</span>
            </Link>
            <nav aria-label="Public navigation" className="flex items-center gap-2 text-sm">
              {authCommitted ? (
                <>
                  <Link className="rounded-lg px-3 py-2 text-[#d8d3c8] transition hover:bg-white/10 hover:text-white" to="/login">
                    Sign in
                  </Link>
                  <Link className="rounded-lg bg-[#f0c36a] px-4 py-2 font-semibold text-[#17110a] transition hover:bg-[#ffd98a]" to="/register">
                    Create account
                  </Link>
                </>
              ) : null}
            </nav>
          </header>

          <div className="grid flex-1 items-center gap-10 py-10 lg:grid-cols-[minmax(0,0.94fr)_minmax(460px,1.06fr)]">
            <div className="max-w-3xl space-y-8">
              <div className="inline-flex items-center gap-2 rounded-lg border border-cyan-200/20 bg-cyan-200/10 px-3 py-2 text-sm text-cyan-100" data-gsap-item>
                <RiShieldCheckLine className="size-4" aria-hidden="true" />
                Multi-tenant AI SaaS command plane
              </div>
              <div className="space-y-5" data-gsap-item>
                <h1 className="font-heading text-5xl font-semibold leading-[1.03] text-white md:text-6xl">Oblivious</h1>
                <p className="text-sm font-semibold text-amber-100">AI workspace framework</p>
                <p className="max-w-2xl text-lg leading-8 text-[#d8d3c8]">
                  Operate Chat, Agent workflows, Knowledge RAG, Relay billing, Marketplace governance, and Admin inspection from one commercial-ready control surface.
                </p>
              </div>
              <div className="flex flex-wrap gap-3" data-gsap-item>
                {authCommitted ? <Link
                  className="inline-flex min-h-[44px] items-center gap-2 rounded-lg bg-[#f0c36a] px-5 py-3 font-semibold text-[#17110a] transition hover:bg-[#ffd98a]"
                  data-gsap-magnetic
                  to="/register"
                >
                  Start commercial workspace
                  <RiArrowRightLine className="size-4" aria-hidden="true" />
                </Link> : null}
                {consoleCommitted ? <Link
                  className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-white/15 bg-white/10 px-5 py-3 font-semibold text-white transition hover:bg-white/15"
                  data-gsap-magnetic
                  to="/console"
                >
                  Open console
                </Link> : null}
              </div>
              <dl className="grid gap-3 sm:grid-cols-2">
                {commandSurfaces.map((surface) => (
                  <div key={surface.label} className="rounded-lg border border-white/10 bg-black/20 p-4 backdrop-blur" data-gsap-item>
                    <dt className="flex items-center gap-2 text-sm text-[#d8d3c8]">
                      <span className="text-cyan-200">{surface.icon}</span>
                      {surface.label}
                    </dt>
                    <dd className="mt-2 text-2xl font-semibold text-white">{surface.value}</dd>
                  </div>
                ))}
              </dl>
            </div>

            <div className="relative" data-gsap-item>
              <div className="absolute -inset-4 rounded-lg border border-cyan-200/10" aria-hidden="true" />
              <section className="relative overflow-hidden rounded-lg border border-white/15 bg-[#17150f]/92 shadow-2xl shadow-black/40">
                <div className="flex items-center justify-between border-b border-white/10 px-5 py-4" data-gsap-item>
                  <div>
                    <p className="text-sm text-[#a9a294]">Commercial journey</p>
                    <h2 className="font-heading text-xl font-semibold text-white">Live operations board</h2>
                  </div>
                  <span className="rounded-lg border border-emerald-300/30 bg-emerald-300/10 px-3 py-1 text-sm text-emerald-100">all gates mapped</span>
                </div>
                <div className="grid gap-px bg-white/10 md:grid-cols-[1.1fr_0.9fr]">
                  <div className="bg-[#17150f] p-5">
                    <div className="space-y-3">
                      {committedRoutes.map((route) => (
                        <Link
                          key={route.path}
                          to={route.path}
                          className="group flex min-h-[76px] items-start justify-between gap-4 rounded-lg border border-white/10 bg-white/[0.04] p-4 transition hover:border-cyan-200/40 hover:bg-cyan-200/10"
                          data-gsap-item
                        >
                          <span>
                            <span className="block font-mono text-sm text-cyan-100">{route.path}</span>
                            <span className="mt-1 block text-sm font-semibold text-white">{route.label}</span>
                            <span className="mt-1 block text-xs leading-5 text-[#bdb5a6]">{route.detail}</span>
                          </span>
                          <RiArrowRightLine className="mt-1 size-4 text-[#f0c36a] transition group-hover:translate-x-1" aria-hidden="true" />
                        </Link>
                      ))}
                    </div>
                  </div>
                  <div className="bg-[#1d1a13] p-5">
                    <div className="space-y-4">
                      <div className="rounded-lg border border-white/10 bg-black/20 p-4" data-gsap-item>
                        <div className="mb-4 flex items-center justify-between text-sm">
                          <span className="text-[#bdb5a6]">Relay settlement</span>
                          <RiFlowChart className="size-5 text-cyan-100" aria-hidden="true" />
                        </div>
                        <div className="space-y-2">
                          <span className="block h-2 rounded-lg bg-cyan-200/70" />
                          <span className="block h-2 w-4/5 rounded-lg bg-amber-200/70" />
                          <span className="block h-2 w-3/5 rounded-lg bg-emerald-200/70" />
                        </div>
                      </div>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-4" data-gsap-item>
                        <div className="mb-3 flex items-center gap-2 text-sm text-[#f7f4ea]">
                          <RiRobot2Line className="size-5 text-amber-100" aria-hidden="true" />
                          Agent approval queue
                        </div>
                        <div className="grid grid-cols-3 gap-2 text-center text-xs">
                          <span className="rounded-lg bg-white/10 p-3">approve</span>
                          <span className="rounded-lg bg-white/10 p-3">retry</span>
                          <span className="rounded-lg bg-white/10 p-3">audit</span>
                        </div>
                      </div>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-4" data-gsap-item>
                        <div className="mb-3 flex items-center gap-2 text-sm text-[#f7f4ea]">
                          <RiStore2Line className="size-5 text-emerald-100" aria-hidden="true" />
                          Marketplace settlement
                        </div>
                        <p className="text-sm leading-6 text-[#bdb5a6]">Paid install, review, refund impact, payout state, and governance events stay visible before operation.</p>
                      </div>
                    </div>
                  </div>
                </div>
              </section>
            </div>
          </div>
        </div>
      </section>

      <section className="bg-[#f4f3ee] px-6 py-10 text-[#181611] lg:px-8">
        <div className="mx-auto grid max-w-7xl gap-4 md:grid-cols-3">
          {[
            ['Frontend scope', 'Public entry, onboarding, Chat, Knowledge, SOLO, Marketplace, Console, Admin.'],
            ['Request proof', 'Core routed pages call the matching `/api/v1/*` APIs and Playwright mocks assert the contracts.'],
            ['Visual proof', 'Browser screenshots are checked for non-black rendering before completion.'],
          ].map(([title, body]) => (
            <article key={title} className="rounded-lg border border-[#d7d2c4] bg-white p-5 shadow-sm" data-gsap-item>
              <h2 className="font-heading text-lg font-semibold">{title}</h2>
              <p className="mt-2 text-sm leading-6 text-[#595346]">{body}</p>
            </article>
          ))}
        </div>
      </section>
    </main>
  );
}
