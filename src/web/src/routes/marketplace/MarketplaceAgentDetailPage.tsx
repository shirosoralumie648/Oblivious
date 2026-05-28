import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { useParams } from 'react-router-dom';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Textarea } from '@/components/ui/textarea';

import { EmptyState } from '../../components/shared/EmptyState';
import { RatingStars } from '../../components/shared/RatingStars';
import { createMarketplaceApi, type AgentReview, type AgentVersion, type MarketplaceAgent } from '../../features/marketplace/api';
import { createHttpClient } from '../../services/http/client';

type DetailState = {
  agent: MarketplaceAgent | null;
  versions: AgentVersion[];
  reviews: AgentReview[];
  loading: boolean;
  error: string | null;
  selectedVersion: string;
  reviewRating: number;
  reviewText: string;
  actionMessage: string | null;
  actionError: { title: string; message?: string } | null;
  installing: boolean;
  submittingReview: boolean;
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; agent: MarketplaceAgent; versions: AgentVersion[]; reviews: AgentReview[] }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_VERSION'; value: string }
  | { type: 'SET_RATING'; value: number }
  | { type: 'SET_REVIEW'; value: string }
  | { type: 'SET_MESSAGE'; value: string | null }
  | { type: 'SET_ACTION_ERROR'; title: string; message?: string }
  | { type: 'SET_INSTALLING'; value: boolean }
  | { type: 'SET_SUBMITTING_REVIEW'; value: boolean };

const initialState: DetailState = {
  agent: null,
  versions: [],
  reviews: [],
  loading: true,
  error: null,
  selectedVersion: '',
  reviewRating: 5,
  reviewText: '',
  actionMessage: null,
  actionError: null,
  installing: false,
  submittingReview: false,
};

function reducer(state: DetailState, action: Action): DetailState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return {
        ...state,
        loading: false,
        error: null,
        agent: action.agent,
        versions: action.versions,
        reviews: action.reviews,
        selectedVersion: action.versions[0]?.id ?? action.versions[0]?.version ?? action.agent.currentVersion ?? '',
      };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'SET_VERSION':
      return { ...state, selectedVersion: action.value };
    case 'SET_RATING':
      return { ...state, reviewRating: action.value };
    case 'SET_REVIEW':
      return { ...state, reviewText: action.value };
    case 'SET_MESSAGE':
      return { ...state, actionError: null, actionMessage: action.value };
    case 'SET_ACTION_ERROR':
      return { ...state, actionError: { title: action.title, message: action.message }, actionMessage: null };
    case 'SET_INSTALLING':
      return { ...state, installing: action.value };
    case 'SET_SUBMITTING_REVIEW':
      return { ...state, submittingReview: action.value };
    default:
      return state;
  }
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== '') {
    return error.message;
  }
  if (typeof error === 'string' && error.trim() !== '') {
    return error;
  }

  return fallback;
}

function priceLabel(agent: MarketplaceAgent) {
  return agent.pricingType === 'free' ? 'Free' : `$${agent.pricingAmount.toFixed(2)}`;
}

export function MarketplaceAgentDetailPage() {
  const { agentId } = useParams();
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createMarketplaceApi(createHttpClient()), []);

  const loadAgent = useCallback(async () => {
    if (!agentId) {
      dispatch({ type: 'LOAD_ERROR', error: 'Agent not found.' });
      return;
    }
    dispatch({ type: 'LOAD_START' });
    try {
      const [agent, versions, reviews] = await Promise.all([
        api.getAgent(agentId),
        api.getVersions(agentId),
        api.getReviews(agentId),
      ]);
      dispatch({ type: 'LOAD_SUCCESS', agent, versions, reviews });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Unable to load agent.' });
    }
  }, [agentId, api]);

  useEffect(() => {
    void loadAgent();
  }, [loadAgent]);

  const handleInstall = async () => {
    if (!agentId) {
      return;
    }
    dispatch({ type: 'SET_INSTALLING', value: true });
    dispatch({ type: 'SET_MESSAGE', value: null });
    try {
      await api.installAgent(agentId, state.selectedVersion || undefined);
      dispatch({ type: 'SET_MESSAGE', value: 'Agent installed.' });
    } catch (error) {
      dispatch({
        type: 'SET_ACTION_ERROR',
        title: 'Unable to install agent.',
        message: getErrorMessage(error, 'Retry the install after checkout or workspace state is ready.'),
      });
    } finally {
      dispatch({ type: 'SET_INSTALLING', value: false });
    }
  };

  const handleReview = async () => {
    if (!agentId) {
      return;
    }
    dispatch({ type: 'SET_SUBMITTING_REVIEW', value: true });
    dispatch({ type: 'SET_MESSAGE', value: null });
    try {
      await api.submitReview(agentId, { rating: state.reviewRating, body: state.reviewText });
      dispatch({ type: 'SET_REVIEW', value: '' });
      dispatch({ type: 'SET_MESSAGE', value: 'Review submitted.' });
      await loadAgent();
    } catch (error) {
      dispatch({
        type: 'SET_ACTION_ERROR',
        title: 'Unable to submit review.',
        message: getErrorMessage(error, 'Retry after the review queue is available.'),
      });
    } finally {
      dispatch({ type: 'SET_SUBMITTING_REVIEW', value: false });
    }
  };

  if (state.loading) {
    return <div className="p-6 text-sm text-muted-foreground">Loading agent...</div>;
  }

  if (state.error) {
    return <EmptyState title={state.error} />;
  }

  if (!state.agent) {
    return <EmptyState title="Agent not found." />;
  }

  const agent = state.agent;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-3">
          <div>
            <h1 className="font-heading text-2xl font-semibold text-foreground">{agent.name}</h1>
            <p className="text-sm text-muted-foreground">by {agent.ownerName ?? agent.ownerID}</p>
          </div>
          <p className="max-w-3xl text-base text-muted-foreground">{agent.description}</p>
          <div className="flex flex-wrap gap-2">
            {agent.categoryName ? <Badge variant="outline">{agent.categoryName}</Badge> : null}
            {agent.tags.map((tag) => <Badge key={tag} variant="secondary">{tag}</Badge>)}
          </div>
        </div>
        <Card className="w-full rounded-lg lg:max-w-xs">
          <CardContent className="space-y-4 p-6">
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Price</span>
              <span className="font-medium">{priceLabel(agent)}</span>
            </div>
            <RatingStars value={agent.ratingAvg ?? agent.rating ?? 0} count={agent.ratingCount} readonly showValue />
            <p className="text-sm text-muted-foreground">{agent.installCount.toLocaleString()} installs</p>
            <p className="text-sm text-muted-foreground">
              {agent.pricingType === 'free'
                ? 'Free agents install directly into the workspace after version selection.'
                : 'Paid installs create a checkout-backed marketplace order before workspace installation.'}
            </p>
            {state.versions.length > 0 ? (
              <select
                aria-label="Agent version"
                value={state.selectedVersion}
                onChange={(event) => dispatch({ type: 'SET_VERSION', value: event.target.value })}
                className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
              >
                {state.versions.map((version) => (
                  <option key={version.id ?? version.version} value={version.id ?? version.version}>
                    {version.version}
                  </option>
                ))}
              </select>
            ) : null}
            <Button type="button" className="min-h-[44px] w-full" disabled={state.installing} onClick={() => void handleInstall()}>
              Install Agent
            </Button>
            {state.actionMessage ? <p className="text-sm text-muted-foreground">{state.actionMessage}</p> : null}
            {state.actionError ? (
              <div role="alert" className="rounded-lg border border-destructive/30 p-3 text-sm text-destructive">
                <p>{state.actionError.title}</p>
                {state.actionError.message ? <p>{state.actionError.message}</p> : null}
              </div>
            ) : null}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card className="rounded-lg">
          <CardHeader>
            <CardTitle>Tools and Examples</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-sm text-muted-foreground">
            <pre className="max-h-48 overflow-auto rounded-lg bg-muted/40 p-3 text-xs">{agent.tools || 'No tools described.'}</pre>
            <pre className="max-h-48 overflow-auto rounded-lg bg-muted/40 p-3 text-xs">{agent.exampleConversations || 'No examples described.'}</pre>
          </CardContent>
        </Card>

        <Card className="rounded-lg">
          <CardHeader>
            <CardTitle>Reviews</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-3">
              <RatingStars value={state.reviewRating} onChange={(value) => dispatch({ type: 'SET_RATING', value })} />
              <Textarea
                aria-label="Review text"
                value={state.reviewText}
                onChange={(event) => dispatch({ type: 'SET_REVIEW', value: event.target.value })}
                placeholder="Share your experience with this agent."
              />
              <Button type="button" className="min-h-[44px]" disabled={state.submittingReview} onClick={() => void handleReview()}>
                Submit Review
              </Button>
            </div>
            <div className="space-y-3">
              {state.reviews.map((review) => (
                <div key={review.id} className="rounded-lg border border-border p-3">
                  <RatingStars value={review.rating} readonly size="sm" showValue />
                  <p className="mt-2 text-sm text-muted-foreground">{review.body ?? review.text ?? 'No review text.'}</p>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
