import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { Link } from 'react-router-dom';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

import { EmptyState } from '../../components/shared/EmptyState';
import { FilterPanel } from '../../components/shared/FilterPanel';
import { RatingStars } from '../../components/shared/RatingStars';
import { SearchBar } from '../../components/shared/SearchBar';
import { createMarketplaceApi, type Category, type CuratedMarketplaceSections, type MarketplaceAgent, type MarketplaceTemplate } from '../../features/marketplace/api';
import { createHttpClient } from '../../services/http/client';

type CuratedSection = {
  key: keyof CuratedMarketplaceSections;
  title: string;
};

type HomeState = {
  agents: MarketplaceAgent[];
  templates: MarketplaceTemplate[];
  categories: Category[];
  curatedSections: CuratedMarketplaceSections;
  loading: boolean;
  error: string | null;
  templateStatus: string | null;
  query: string;
  selectedCategories: string[];
  selectedTags: string[];
  minRating: number;
  priceFilter: 'all' | 'free' | 'paid';
  sort: 'relevance' | 'rating' | 'installs' | 'newest' | 'popular' | 'recommended';
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; agents: MarketplaceAgent[]; templates: MarketplaceTemplate[]; categories: Category[]; curatedSections: CuratedMarketplaceSections }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_TEMPLATE_STATUS'; value: string | null }
  | { type: 'SET_QUERY'; value: string }
  | { type: 'SET_CATEGORIES'; value: string[] }
  | { type: 'SET_TAGS'; value: string[] }
  | { type: 'SET_RATING'; value: number }
  | { type: 'SET_PRICE'; value: 'all' | 'free' | 'paid' }
  | { type: 'SET_SORT'; value: HomeState['sort'] };

const initialState: HomeState = {
  agents: [],
  templates: [],
  categories: [],
  curatedSections: { popular: [], topRated: [], recent: [] },
  loading: true,
  error: null,
  templateStatus: null,
  query: '',
  selectedCategories: [],
  selectedTags: [],
  minRating: 0,
  priceFilter: 'all',
  sort: 'recommended',
};

function reducer(state: HomeState, action: Action): HomeState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return {
        ...state,
        loading: false,
        error: null,
        agents: action.agents,
        templates: action.templates,
        categories: action.categories,
        curatedSections: action.curatedSections,
      };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'SET_TEMPLATE_STATUS':
      return { ...state, templateStatus: action.value };
    case 'SET_QUERY':
      return { ...state, query: action.value };
    case 'SET_CATEGORIES':
      return { ...state, selectedCategories: action.value };
    case 'SET_TAGS':
      return { ...state, selectedTags: action.value };
    case 'SET_RATING':
      return { ...state, minRating: action.value };
    case 'SET_PRICE':
      return { ...state, priceFilter: action.value };
    case 'SET_SORT':
      return { ...state, sort: action.value };
    default:
      return state;
  }
}

function priceLabel(agent: MarketplaceAgent) {
  if (agent.pricingType === 'free') {
    return 'Free';
  }
  return `$${agent.pricingAmount.toFixed(2)}`;
}

const curatedSectionConfig: CuratedSection[] = [
  { key: 'popular', title: 'Popular' },
  { key: 'topRated', title: 'Top rated' },
  { key: 'recent', title: 'New arrivals' },
];

function hasCuratedAgents(sections: CuratedMarketplaceSections) {
  return curatedSectionConfig.some((section) => sections[section.key].length > 0);
}

function AgentCard({ agent, compact = false }: { agent: MarketplaceAgent; compact?: boolean }) {
  const recommendationScore = agent.recommendation ? Math.round(agent.recommendation.score * 100) : null;

  return (
    <Card className="rounded-lg">
      <CardHeader>
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle className={compact ? 'text-base' : 'text-lg'}>{agent.name}</CardTitle>
            <p className="text-sm text-muted-foreground">by {agent.ownerName ?? agent.ownerID}</p>
          </div>
          <Badge variant={agent.pricingType === 'free' ? 'secondary' : 'outline'}>{priceLabel(agent)}</Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className={compact ? 'line-clamp-2 min-h-[40px] text-sm text-muted-foreground' : 'line-clamp-3 text-sm text-muted-foreground'}>
          {agent.description}
        </p>
        {agent.recommendation ? (
          <div className="rounded-lg border border-border bg-muted/30 px-3 py-2">
            <div className="flex items-center justify-between gap-3">
              <span className="text-xs font-medium text-foreground">Recommended</span>
              {recommendationScore !== null ? <span className="text-xs text-muted-foreground">{recommendationScore}% match</span> : null}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">{agent.recommendation.reason}</p>
          </div>
        ) : null}
        <div className="flex flex-wrap gap-2">
          {agent.categoryName ? <Badge variant="outline">{agent.categoryName}</Badge> : null}
          {agent.tags.slice(0, compact ? 3 : 4).map((tag) => <Badge key={tag} variant="secondary">{tag}</Badge>)}
        </div>
        <div className="flex items-center justify-between gap-3">
          <RatingStars value={agent.ratingAvg ?? agent.rating ?? 0} count={agent.ratingCount} readonly showValue />
          <span className="text-sm text-muted-foreground">{agent.installCount.toLocaleString()} installs</span>
        </div>
        <Button type="button" variant="outline" className="min-h-[44px] w-full" asChild>
          <Link to={`/marketplace/agents/${agent.id}`}>View Agent</Link>
        </Button>
      </CardContent>
    </Card>
  );
}

function CuratedLoadingSections() {
  return (
    <div className="space-y-3" role="status" aria-label="Loading marketplace sections">
      <div className="h-5 w-36 rounded bg-muted" />
      <div className="grid gap-3 xl:grid-cols-3">
        {[0, 1, 2].map((item) => (
          <div key={item} className="h-44 rounded-lg bg-muted/60" />
        ))}
      </div>
    </div>
  );
}

export function MarketplaceHomePage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createMarketplaceApi(createHttpClient()), []);

  const availableTags = useMemo(
    () => Array.from(new Set(state.agents.flatMap((agent) => agent.tags))).sort(),
    [state.agents]
  );
  const showCuratedSections = hasCuratedAgents(state.curatedSections);

  const loadAgents = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const [searchResult, templateResult, categories, curatedSections] = await Promise.all([
        api.searchAgents({
          query: state.query,
          categorySlug: state.selectedCategories[0],
          tags: state.selectedTags,
          minRating: state.minRating,
          priceFilter: state.priceFilter,
          sort: state.sort,
          limit: 50,
        }),
        api.listTemplates({ query: state.query, limit: 6 }),
        api.getCategories(),
        api.getCuratedSections().catch(() => ({ popular: [], topRated: [], recent: [] })),
      ]);
      dispatch({ type: 'LOAD_SUCCESS', agents: searchResult.agents, templates: templateResult.templates, categories, curatedSections });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading marketplace agents.' });
    }
  }, [api, state.minRating, state.priceFilter, state.query, state.selectedCategories, state.selectedTags, state.sort]);

  const useTemplate = useCallback(async (template: MarketplaceTemplate) => {
    dispatch({ type: 'SET_TEMPLATE_STATUS', value: null });
    try {
      await api.installTemplate(template.id);
      dispatch({ type: 'SET_TEMPLATE_STATUS', value: 'Template ready to use.' });
    } catch (error) {
      dispatch({ type: 'SET_TEMPLATE_STATUS', value: error instanceof Error ? error.message : 'Unable to use template.' });
    }
  }, [api]);

  useEffect(() => {
    void loadAgents();
  }, [loadAgents]);

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 className="font-heading text-2xl font-semibold text-foreground">Agent Marketplace</h1>
          <p className="text-sm text-muted-foreground">Browse, install, and publish reusable agents.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="outline" className="min-h-[44px]" asChild>
            <Link to="/marketplace/my-agents">My Agents</Link>
          </Button>
          <Button type="button" className="min-h-[44px]" asChild>
            <Link to="/marketplace/publish">Publish Agent</Link>
          </Button>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-[280px_minmax(0,1fr)]">
        <FilterPanel
          categories={state.categories.map((category) => ({ slug: category.slug, name: category.name, count: category.agentCount }))}
          selectedCategories={state.selectedCategories}
          onCategoryChange={(value) => dispatch({ type: 'SET_CATEGORIES', value })}
          selectedTags={state.selectedTags}
          availableTags={availableTags}
          onTagsChange={(value) => dispatch({ type: 'SET_TAGS', value })}
          minRating={state.minRating}
          onRatingChange={(value) => dispatch({ type: 'SET_RATING', value })}
          priceFilter={state.priceFilter}
          onPriceFilterChange={(value) => dispatch({ type: 'SET_PRICE', value })}
        />

        <section className="space-y-4">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
            <SearchBar value={state.query} onChange={(value) => dispatch({ type: 'SET_QUERY', value })} placeholder="Search agents..." />
            <select
              aria-label="Marketplace sort"
              value={state.sort}
              onChange={(event) => dispatch({ type: 'SET_SORT', value: event.target.value as HomeState['sort'] })}
              className="min-h-[44px] rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
            >
              <option value="recommended">Recommended</option>
              <option value="relevance">Relevance</option>
              <option value="rating">Rating</option>
              <option value="installs">Installs</option>
              <option value="newest">Newest</option>
              <option value="popular">Popular</option>
            </select>
          </div>

          {state.error ? (
            <div className="rounded-lg border border-destructive/30 bg-card p-6 text-sm text-destructive" role="alert">
              {state.error}
            </div>
          ) : null}

          {state.loading ? <CuratedLoadingSections /> : null}

          {!state.loading && !state.error && showCuratedSections ? (
            <div className="space-y-5">
              {curatedSectionConfig.map((section) => {
                const sectionAgents = state.curatedSections[section.key];
                if (sectionAgents.length === 0) {
                  return null;
                }

                return (
                  <section key={section.key} className="space-y-3" aria-labelledby={`marketplace-${section.key}-title`}>
                    <h2 id={`marketplace-${section.key}-title`} className="font-heading text-lg font-semibold text-foreground">
                      {section.title}
                    </h2>
                    <div className="grid gap-3 xl:grid-cols-3">
                      {sectionAgents.map((agent) => <AgentCard key={agent.id} agent={agent} compact />)}
                    </div>
                  </section>
                );
              })}
            </div>
          ) : null}

          {state.templates.length > 0 ? (
            <section className="space-y-3" aria-labelledby="marketplace-templates-title">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h2 id="marketplace-templates-title" className="font-heading text-lg font-semibold text-foreground">Templates</h2>
                  <p className="text-sm text-muted-foreground">Reusable Bot, workflow, and plugin starting points.</p>
                </div>
              </div>
              {state.templateStatus ? (
                <div className="rounded-lg border border-border bg-card px-4 py-3 text-sm text-foreground" role="status">
                  {state.templateStatus}
                </div>
              ) : null}
              <div className="grid gap-3 xl:grid-cols-3">
                {state.templates.map((template) => (
                  <Card key={template.id} className="rounded-lg">
                    <CardHeader>
                      <div className="flex items-start justify-between gap-3">
                        <CardTitle className="text-base">{template.name}</CardTitle>
                        <Badge variant="outline">{template.type}</Badge>
                      </div>
                    </CardHeader>
                    <CardContent className="space-y-3">
                      <p className="line-clamp-2 min-h-[40px] text-sm text-muted-foreground">{template.description}</p>
                      <div className="flex flex-wrap gap-2">
                        {template.category ? <Badge variant="outline">{template.category}</Badge> : null}
                        {template.tags.slice(0, 3).map((tag) => <Badge key={tag} variant="secondary">{tag}</Badge>)}
                      </div>
                      <div className="text-xs text-muted-foreground">{template.downloadsCount.toLocaleString()} uses</div>
                      <Button type="button" variant="outline" className="min-h-[44px] w-full" onClick={() => void useTemplate(template)}>
                        Use {template.name}
                      </Button>
                    </CardContent>
                  </Card>
                ))}
              </div>
            </section>
          ) : null}

          {!state.loading && !state.error && state.agents.length === 0 ? (
            <EmptyState title="No agents found -- Adjust your filters or publish the first agent." />
          ) : null}

          {state.agents.length > 0 ? (
            <div className="grid gap-4 xl:grid-cols-2">
              {state.agents.map((agent) => <AgentCard key={agent.id} agent={agent} />)}
            </div>
          ) : null}
        </section>
      </div>
    </div>
  );
}
