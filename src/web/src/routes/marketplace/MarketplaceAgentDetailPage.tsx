import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { useParams } from 'react-router-dom';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Textarea } from '@/components/ui/textarea';

import { EmptyState } from '../../components/shared/EmptyState';
import { RatingStars } from '../../components/shared/RatingStars';
import {
  createMarketplaceApi,
  getMarketplaceCheckoutUrl,
  isMarketplaceCheckoutResponse,
  type AgentReview,
  type AgentVersion,
  type MarketplaceAgent,
} from '../../features/marketplace/api';
import { createHttpClient } from '../../services/http/client';

type DetailState = {
  agent: MarketplaceAgent | null;
  versions: AgentVersion[];
  reviews: AgentReview[];
  loading: boolean;
  error: string | null;
  selectedVersion: string;
  paymentProvider: string;
  reviewRating: number;
  reviewText: string;
  appealReason: string;
  abuseReason: string;
  abuseDetails: string;
  actionMessage: string | null;
  checkoutUrl: string | null;
  actionError: { title: string; message?: string } | null;
  installing: boolean;
  submittingReview: boolean;
  submittingAppeal: boolean;
  submittingAbuseReport: boolean;
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; agent: MarketplaceAgent; versions: AgentVersion[]; reviews: AgentReview[] }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_VERSION'; value: string }
  | { type: 'SET_PAYMENT_PROVIDER'; value: string }
  | { type: 'SET_RATING'; value: number }
  | { type: 'SET_REVIEW'; value: string }
  | { type: 'SET_APPEAL_REASON'; value: string }
  | { type: 'SET_ABUSE_REASON'; value: string }
  | { type: 'SET_ABUSE_DETAILS'; value: string }
  | { type: 'SET_MESSAGE'; value: string | null }
  | { type: 'SET_CHECKOUT'; url: string }
  | { type: 'SET_ACTION_ERROR'; title: string; message?: string }
  | { type: 'SET_INSTALLING'; value: boolean }
  | { type: 'SET_SUBMITTING_REVIEW'; value: boolean }
  | { type: 'SET_SUBMITTING_APPEAL'; value: boolean }
  | { type: 'SET_SUBMITTING_ABUSE_REPORT'; value: boolean };

const initialState: DetailState = {
  agent: null,
  versions: [],
  reviews: [],
  loading: true,
  error: null,
  selectedVersion: '',
  paymentProvider: 'stripe',
  reviewRating: 5,
  reviewText: '',
  appealReason: '',
  abuseReason: '',
  abuseDetails: '',
  actionMessage: null,
  checkoutUrl: null,
  actionError: null,
  installing: false,
  submittingReview: false,
  submittingAppeal: false,
  submittingAbuseReport: false,
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
    case 'SET_PAYMENT_PROVIDER':
      return { ...state, paymentProvider: action.value };
    case 'SET_RATING':
      return { ...state, reviewRating: action.value };
    case 'SET_REVIEW':
      return { ...state, reviewText: action.value };
    case 'SET_APPEAL_REASON':
      return { ...state, appealReason: action.value };
    case 'SET_ABUSE_REASON':
      return { ...state, abuseReason: action.value };
    case 'SET_ABUSE_DETAILS':
      return { ...state, abuseDetails: action.value };
    case 'SET_MESSAGE':
      return { ...state, actionError: null, actionMessage: action.value, checkoutUrl: null };
    case 'SET_CHECKOUT':
      return { ...state, actionError: null, actionMessage: 'Checkout session ready.', checkoutUrl: action.url };
    case 'SET_ACTION_ERROR':
      return { ...state, actionError: { title: action.title, message: action.message }, actionMessage: null, checkoutUrl: null };
    case 'SET_INSTALLING':
      return { ...state, installing: action.value };
    case 'SET_SUBMITTING_REVIEW':
      return { ...state, submittingReview: action.value };
    case 'SET_SUBMITTING_APPEAL':
      return { ...state, submittingAppeal: action.value };
    case 'SET_SUBMITTING_ABUSE_REPORT':
      return { ...state, submittingAbuseReport: action.value };
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

function isPaidAgent(agent: MarketplaceAgent) {
  return agent.pricingType !== 'free' && agent.pricingAmount > 0;
}

function paymentProviderLabel(provider: string) {
  switch (provider) {
    case 'alipay':
      return 'Alipay';
    case 'wechatpay':
      return 'WeChat Pay';
    default:
      return 'Stripe';
  }
}

function normalizedPaymentProviders(agent: MarketplaceAgent) {
  if (agent.paymentProviders === undefined) {
    return [{ name: 'stripe' }];
  }

  return agent.paymentProviders
    .map((provider) => ({ name: provider.name.trim().toLowerCase() }))
    .filter((provider) => provider.name !== '');
}

function selectedPaymentProviderForAgent(agent: MarketplaceAgent, selectedProvider: string) {
  const providers = normalizedPaymentProviders(agent);
  return providers.some((provider) => provider.name === selectedProvider)
    ? selectedProvider
    : providers[0]?.name ?? '';
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
      const paymentProvider = state.agent && isPaidAgent(state.agent)
        ? selectedPaymentProviderForAgent(state.agent, state.paymentProvider)
        : '';
      const installResult =
        state.agent && isPaidAgent(state.agent)
          ? await api.installAgent(agentId, state.selectedVersion || undefined, paymentProvider || undefined)
          : await api.installAgent(agentId, state.selectedVersion || undefined);
      const checkoutUrl = getMarketplaceCheckoutUrl(installResult);
      if (isMarketplaceCheckoutResponse(installResult) && checkoutUrl) {
        dispatch({ type: 'SET_CHECKOUT', url: checkoutUrl });
      } else {
        dispatch({ type: 'SET_MESSAGE', value: 'Agent installed.' });
      }
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

  const handleAppeal = async () => {
    if (!agentId) {
      return;
    }
    const reason = state.appealReason.trim();
    if (reason === '') {
      return;
    }

    dispatch({ type: 'SET_SUBMITTING_APPEAL', value: true });
    dispatch({ type: 'SET_MESSAGE', value: null });
    try {
      await api.appealAgent(agentId, { reason });
      dispatch({ type: 'SET_APPEAL_REASON', value: '' });
      dispatch({ type: 'SET_MESSAGE', value: 'Appeal submitted.' });
      await loadAgent();
    } catch (error) {
      dispatch({
        type: 'SET_ACTION_ERROR',
        title: 'Unable to submit appeal.',
        message: getErrorMessage(error, 'Retry after marketplace governance is available.'),
      });
    } finally {
      dispatch({ type: 'SET_SUBMITTING_APPEAL', value: false });
    }
  };

  const handleReportAbuse = async () => {
    if (!agentId) {
      return;
    }
    const reason = state.abuseReason.trim();
    if (reason === '') {
      return;
    }
    const details = state.abuseDetails.trim();

    dispatch({ type: 'SET_SUBMITTING_ABUSE_REPORT', value: true });
    dispatch({ type: 'SET_MESSAGE', value: null });
    try {
      await api.reportAbuse(agentId, { reason, ...(details === '' ? {} : { details }) });
      dispatch({ type: 'SET_ABUSE_REASON', value: '' });
      dispatch({ type: 'SET_ABUSE_DETAILS', value: '' });
      dispatch({ type: 'SET_MESSAGE', value: 'Abuse report submitted.' });
    } catch (error) {
      dispatch({
        type: 'SET_ACTION_ERROR',
        title: 'Unable to report abuse.',
        message: getErrorMessage(error, 'Retry after marketplace governance is available.'),
      });
    } finally {
      dispatch({ type: 'SET_SUBMITTING_ABUSE_REPORT', value: false });
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
  const paidAgent = isPaidAgent(agent);
  const paymentProviders = paidAgent ? normalizedPaymentProviders(agent) : [];
  const selectedPaymentProvider = paidAgent ? selectedPaymentProviderForAgent(agent, state.paymentProvider) : '';
  const checkoutLabel =
    selectedPaymentProvider === 'stripe' ? 'Continue checkout' : `Continue ${paymentProviderLabel(selectedPaymentProvider)} checkout`;

  return (
    <div className="min-w-0 space-y-6">
      <div className="flex min-w-0 flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0 space-y-3">
          <div className="min-w-0">
            <h1 className="break-words font-heading text-2xl font-semibold text-foreground [overflow-wrap:anywhere]">{agent.name}</h1>
            <p className="min-w-0 break-words text-sm text-muted-foreground [overflow-wrap:anywhere]">by {agent.ownerName ?? agent.ownerID}</p>
          </div>
          <p className="max-w-3xl break-words text-base text-muted-foreground [overflow-wrap:anywhere]">{agent.description}</p>
          <div className="flex min-w-0 flex-wrap gap-2">
            {agent.categoryName ? (
              <Badge variant="outline" className="max-w-full whitespace-normal break-words [overflow-wrap:anywhere]">
                {agent.categoryName}
              </Badge>
            ) : null}
            {agent.tags.map((tag) => (
              <Badge key={tag} variant="secondary" className="max-w-full whitespace-normal break-words [overflow-wrap:anywhere]">
                {tag}
              </Badge>
            ))}
          </div>
        </div>
        <Card className="w-full min-w-0 max-w-full rounded-lg lg:max-w-xs">
          <CardContent className="min-w-0 space-y-4 p-6">
            <div className="flex min-w-0 items-center justify-between gap-3">
              <span className="text-sm text-muted-foreground">Price</span>
              <span className="font-medium">{priceLabel(agent)}</span>
            </div>
            <RatingStars value={agent.ratingAvg ?? agent.rating ?? 0} count={agent.ratingCount} readonly showValue />
            <p className="break-words text-sm text-muted-foreground [overflow-wrap:anywhere]">{agent.installCount.toLocaleString()} installs</p>
            <p className="break-words text-sm text-muted-foreground [overflow-wrap:anywhere]">
              {agent.pricingType === 'free'
                ? 'Free agents install directly into the workspace after version selection.'
                : 'Paid installs create a checkout-backed marketplace order before workspace installation.'}
            </p>
            {state.versions.length > 0 ? (
              <select
                aria-label="Agent version"
                value={state.selectedVersion}
                onChange={(event) => dispatch({ type: 'SET_VERSION', value: event.target.value })}
                className="min-h-[44px] w-full min-w-0 rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
              >
                {state.versions.map((version) => (
                  <option key={version.id ?? version.version} value={version.id ?? version.version}>
                    {version.version}
                  </option>
                ))}
              </select>
            ) : null}
            {paidAgent && paymentProviders.length > 0 ? (
              <select
                aria-label="Payment provider"
                value={selectedPaymentProvider}
                onChange={(event) => dispatch({ type: 'SET_PAYMENT_PROVIDER', value: event.target.value })}
                className="min-h-[44px] w-full min-w-0 rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
              >
                {paymentProviders.map((provider) => (
                  <option key={provider.name} value={provider.name}>{paymentProviderLabel(provider.name)}</option>
                ))}
              </select>
            ) : null}
            {paidAgent && paymentProviders.length === 0 ? (
              <p role="status" className="break-words rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-900 [overflow-wrap:anywhere]">
                Payment provider checkout is not configured for this paid agent.
              </p>
            ) : null}
            <Button type="button" className="min-h-[44px] w-full" disabled={state.installing || (paidAgent && paymentProviders.length === 0)} onClick={() => void handleInstall()}>
              Install Agent
            </Button>
            {state.actionMessage ? <p className="break-words text-sm text-muted-foreground [overflow-wrap:anywhere]">{state.actionMessage}</p> : null}
            {state.checkoutUrl ? (
              <Button asChild className="min-h-[44px] w-full" variant="secondary">
                <a href={state.checkoutUrl}>{checkoutLabel}</a>
              </Button>
            ) : null}
            {state.actionError ? (
              <div role="alert" className="break-words rounded-lg border border-destructive/30 p-3 text-sm text-destructive [overflow-wrap:anywhere]">
                <p>{state.actionError.title}</p>
                {state.actionError.message ? <p>{state.actionError.message}</p> : null}
              </div>
            ) : null}
          </CardContent>
        </Card>
      </div>

      <div className="grid min-w-0 gap-6 lg:grid-cols-2">
        <Card className="min-w-0 rounded-lg">
          <CardHeader className="min-w-0">
            <CardTitle className="break-words [overflow-wrap:anywhere]">Tools and Examples</CardTitle>
          </CardHeader>
          <CardContent className="min-w-0 space-y-4 text-sm text-muted-foreground">
            <pre className="max-h-48 max-w-full overflow-auto whitespace-pre-wrap break-words rounded-lg bg-muted/40 p-3 text-xs [overflow-wrap:anywhere]">{agent.tools || 'No tools described.'}</pre>
            <pre className="max-h-48 max-w-full overflow-auto whitespace-pre-wrap break-words rounded-lg bg-muted/40 p-3 text-xs [overflow-wrap:anywhere]">{agent.exampleConversations || 'No examples described.'}</pre>
          </CardContent>
        </Card>

        <Card className="min-w-0 rounded-lg">
          <CardHeader className="min-w-0">
            <CardTitle className="break-words [overflow-wrap:anywhere]">Reviews</CardTitle>
          </CardHeader>
          <CardContent className="min-w-0 space-y-4">
            <div className="min-w-0 space-y-3">
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
            <div className="min-w-0 space-y-3">
              {state.reviews.map((review) => (
                <div key={review.id} className="min-w-0 rounded-lg border border-border p-3">
                  <RatingStars value={review.rating} readonly size="sm" showValue />
                  <p className="mt-2 min-w-0 break-words text-sm text-muted-foreground [overflow-wrap:anywhere]">{review.body ?? review.text ?? 'No review text.'}</p>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        <Card className="min-w-0 rounded-lg">
          <CardHeader className="min-w-0">
            <CardTitle className="break-words [overflow-wrap:anywhere]">Marketplace Governance</CardTitle>
          </CardHeader>
          <CardContent className="min-w-0 space-y-6">
            <div className="min-w-0 space-y-3">
              <Textarea
                aria-label="Appeal reason"
                value={state.appealReason}
                onChange={(event) => dispatch({ type: 'SET_APPEAL_REASON', value: event.target.value })}
                placeholder="Describe the change that resolves the takedown or review issue."
              />
              <Button
                type="button"
                className="min-h-[44px]"
                disabled={state.submittingAppeal || state.appealReason.trim() === ''}
                onClick={() => void handleAppeal()}
              >
                Submit Appeal
              </Button>
            </div>
            <div className="min-w-0 space-y-3">
              <Textarea
                aria-label="Abuse reason"
                value={state.abuseReason}
                onChange={(event) => dispatch({ type: 'SET_ABUSE_REASON', value: event.target.value })}
                placeholder="Summarize the policy or safety issue."
              />
              <Textarea
                aria-label="Abuse details"
                value={state.abuseDetails}
                onChange={(event) => dispatch({ type: 'SET_ABUSE_DETAILS', value: event.target.value })}
                placeholder="Add evidence or reproduction details."
              />
              <Button
                type="button"
                className="min-h-[44px]"
                disabled={state.submittingAbuseReport || state.abuseReason.trim() === ''}
                onClick={() => void handleReportAbuse()}
              >
                Report Abuse
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
