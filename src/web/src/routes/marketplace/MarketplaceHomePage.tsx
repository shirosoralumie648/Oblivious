import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { Link } from 'react-router-dom';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

import { EmptyState } from '../../components/shared/EmptyState';
import { FilterPanel } from '../../components/shared/FilterPanel';
import { RatingStars } from '../../components/shared/RatingStars';
import { SearchBar } from '../../components/shared/SearchBar';
import { createMarketplaceApi, type Category, type MarketplaceAgent } from '../../features/marketplace/api';
import { createHttpClient } from '../../services/http/client';

type HomeState = {
  agents: MarketplaceAgent[];
  categories: Category[];
  loading: boolean;
  error: string | null;
  query: string;
  selectedCategories: string[];
  selectedTags: string[];
  minRating: number;
  priceFilter: 'all' | 'free' | 'paid';
  sort: 'relevance' | 'rating' | 'installs' | 'newest' | 'popular' | 'recommended';
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; agents: MarketplaceAgent[]; categories: Category[] }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_QUERY'; value: string }
  | { type: 'SET_CATEGORIES'; value: string[] }
  | { type: 'SET_TAGS'; value: string[] }
  | { type: 'SET_RATING'; value: number }
  | { type: 'SET_PRICE'; value: 'all' | 'free' | 'paid' }
  | { type: 'SET_SORT'; value: HomeState['sort'] };

const initialState: HomeState = {
  agents: [],
  categories: [],
  loading: true,
  error: null,
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
      return { ...state, loading: false, error: null, agents: action.agents, categories: action.categories };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
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

export function MarketplaceHomePage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createMarketplaceApi(createHttpClient()), []);

  const availableTags = useMemo(
    () => Array.from(new Set(state.agents.flatMap((agent) => agent.tags))).sort(),
    [state.agents]
  );

  const loadAgents = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const [searchResult, categories] = await Promise.all([
        api.searchAgents({
          query: state.query,
          categorySlug: state.selectedCategories[0],
          tags: state.selectedTags,
          minRating: state.minRating,
          priceFilter: state.priceFilter,
          sort: state.sort,
          limit: 50,
        }),
        api.getCategories(),
      ]);
      dispatch({ type: 'LOAD_SUCCESS', agents: searchResult.agents, categories });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading marketplace agents.' });
    }
  }, [api, state.minRating, state.priceFilter, state.query, state.selectedCategories, state.selectedTags, state.sort]);

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

          {!state.loading && !state.error && state.agents.length === 0 ? (
            <EmptyState title="No agents found -- Adjust your filters or publish the first agent." />
          ) : null}

          <div className="grid gap-4 xl:grid-cols-2">
            {state.agents.map((agent) => (
              <Card key={agent.id} className="rounded-lg">
                <CardHeader>
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <CardTitle className="text-lg">{agent.name}</CardTitle>
                      <p className="text-sm text-muted-foreground">by {agent.ownerName ?? agent.ownerID}</p>
                    </div>
                    <Badge variant={agent.pricingType === 'free' ? 'secondary' : 'outline'}>{priceLabel(agent)}</Badge>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  <p className="line-clamp-3 text-sm text-muted-foreground">{agent.description}</p>
                  <div className="flex flex-wrap gap-2">
                    {agent.categoryName ? <Badge variant="outline">{agent.categoryName}</Badge> : null}
                    {agent.tags.slice(0, 4).map((tag) => <Badge key={tag} variant="secondary">{tag}</Badge>)}
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
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}
