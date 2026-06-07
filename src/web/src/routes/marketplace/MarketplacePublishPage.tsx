import { useCallback, useEffect, useMemo, useReducer } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';

import {
  createMarketplaceApi,
  getAutomatedReviewRejection,
  type AgentPublishRequest,
  type AutomatedReviewResult,
  type Category,
} from '../../features/marketplace/api';
import { createHttpClient } from '../../services/http/client';

type PublishForm = AgentPublishRequest;

type PublishState = {
  categories: Category[];
  form: PublishForm;
  loading: boolean;
  error: string | null;
  automatedReview: AutomatedReviewResult | null;
  success: string | null;
};

type Action =
  | { type: 'LOAD_CATEGORIES'; categories: Category[] }
  | { type: 'FIELD'; field: keyof PublishForm; value: string | string[] | number }
  | { type: 'SUBMIT_START' }
  | { type: 'SUBMIT_SUCCESS'; message: string }
  | { type: 'SUBMIT_ERROR'; error: string; automatedReview?: AutomatedReviewResult | null };

const initialForm: PublishForm = {
  name: '',
  description: '',
  categoryID: '',
  tags: [],
  tools: '',
  exampleConversations: '',
  systemPrompt: '',
  visibility: 'public',
  pricingType: 'free',
  pricingAmount: 0,
  version: '1.0.0',
  changelog: '',
};

const initialState: PublishState = {
  categories: [],
  form: initialForm,
  loading: false,
  error: null,
  automatedReview: null,
  success: null,
};

function reducer(state: PublishState, action: Action): PublishState {
  switch (action.type) {
    case 'LOAD_CATEGORIES':
      return { ...state, categories: action.categories };
    case 'FIELD':
      return { ...state, form: { ...state.form, [action.field]: action.value }, success: null };
    case 'SUBMIT_START':
      return { ...state, loading: true, error: null, automatedReview: null, success: null };
    case 'SUBMIT_SUCCESS':
      return { ...state, loading: false, error: null, automatedReview: null, success: action.message };
    case 'SUBMIT_ERROR':
      return { ...state, loading: false, error: action.error, automatedReview: action.automatedReview ?? null, success: null };
    default:
      return state;
  }
}

function publishPayload(form: PublishForm): AgentPublishRequest {
  return {
    ...form,
    name: form.name.trim(),
    description: form.description.trim(),
    tags: form.tags.map((tag) => tag.trim()).filter(Boolean),
    pricingAmount: Number(form.pricingAmount) || 0,
    version: form.version.trim(),
  };
}

export function MarketplacePublishPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createMarketplaceApi(createHttpClient()), []);

  const loadCategories = useCallback(async () => {
    const categories = await api.getCategories();
    dispatch({ type: 'LOAD_CATEGORIES', categories });
  }, [api]);

  useEffect(() => {
    void loadCategories();
  }, [loadCategories]);

  const handleSubmit = async () => {
    const payload = publishPayload(state.form);
    if (!payload.name || !payload.description || !payload.version) {
      dispatch({ type: 'SUBMIT_ERROR', error: 'Name, description, and version are required.' });
      return;
    }
    if (payload.pricingAmount < 0) {
      dispatch({ type: 'SUBMIT_ERROR', error: 'Price cannot be negative.' });
      return;
    }

    dispatch({ type: 'SUBMIT_START' });
    try {
      await api.publishAgent(payload);
      dispatch({
        type: 'SUBMIT_SUCCESS',
        message:
          payload.pricingType === 'free'
            ? 'Agent submitted for review.'
            : 'Agent submitted for review. Paid installs remain checkout-backed until approval and settlement evidence exist.',
      });
    } catch (error) {
      const automatedReview = getAutomatedReviewRejection(error);
      dispatch({
        type: 'SUBMIT_ERROR',
        error: automatedReview ? 'Automated review rejected this submission.' : error instanceof Error ? error.message : 'Unable to publish agent.',
        automatedReview,
      });
    }
  };

  return (
    <div className="max-w-4xl space-y-6">
      <div>
        <h1 className="font-heading text-2xl font-semibold text-foreground">Publish Agent</h1>
        <p className="text-sm text-muted-foreground">Submit an agent for marketplace review.</p>
        <p className="text-sm text-muted-foreground">
          Public and paid submissions enter review before paid operation; paid installs stay checkout-backed until approval and settlement evidence exist.
        </p>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <div className="space-y-2">
          <label htmlFor="agent-name" className="text-sm font-medium">Name</label>
          <Input id="agent-name" value={state.form.name} onChange={(event) => dispatch({ type: 'FIELD', field: 'name', value: event.target.value })} />
        </div>
        <div className="space-y-2">
          <label htmlFor="agent-category" className="text-sm font-medium">Category</label>
          <select
            id="agent-category"
            value={state.form.categoryID ?? ''}
            onChange={(event) => dispatch({ type: 'FIELD', field: 'categoryID', value: event.target.value })}
            className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
          >
            <option value="">Select category</option>
            {state.categories.map((category) => (
              <option key={category.id} value={category.id}>{category.name}</option>
            ))}
          </select>
        </div>
      </div>

      <div className="space-y-2">
        <label htmlFor="agent-description" className="text-sm font-medium">Description</label>
        <Textarea id="agent-description" value={state.form.description} onChange={(event) => dispatch({ type: 'FIELD', field: 'description', value: event.target.value })} />
      </div>

      <div className="space-y-2">
        <label htmlFor="agent-tags" className="text-sm font-medium">Tags</label>
        <Input id="agent-tags" placeholder="research, writing" value={state.form.tags.join(', ')} onChange={(event) => dispatch({ type: 'FIELD', field: 'tags', value: event.target.value.split(',') })} />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <div className="space-y-2">
          <label htmlFor="agent-tools" className="text-sm font-medium">Tools</label>
          <Textarea id="agent-tools" value={state.form.tools} onChange={(event) => dispatch({ type: 'FIELD', field: 'tools', value: event.target.value })} />
        </div>
        <div className="space-y-2">
          <label htmlFor="agent-examples" className="text-sm font-medium">Example Conversations</label>
          <Textarea id="agent-examples" value={state.form.exampleConversations} onChange={(event) => dispatch({ type: 'FIELD', field: 'exampleConversations', value: event.target.value })} />
        </div>
      </div>

      <div className="space-y-2">
        <label htmlFor="agent-system-prompt" className="text-sm font-medium">System Prompt</label>
        <Textarea id="agent-system-prompt" value={state.form.systemPrompt} onChange={(event) => dispatch({ type: 'FIELD', field: 'systemPrompt', value: event.target.value })} />
      </div>

      <div className="grid gap-4 lg:grid-cols-4">
        <div className="space-y-2">
          <label htmlFor="agent-visibility" className="text-sm font-medium">Visibility</label>
          <select id="agent-visibility" value={state.form.visibility} onChange={(event) => dispatch({ type: 'FIELD', field: 'visibility', value: event.target.value })} className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground">
            <option value="public">Public</option>
            <option value="unlisted">Unlisted</option>
            <option value="private">Private</option>
          </select>
        </div>
        <div className="space-y-2">
          <label htmlFor="agent-pricing-type" className="text-sm font-medium">Pricing</label>
          <select id="agent-pricing-type" value={state.form.pricingType} onChange={(event) => dispatch({ type: 'FIELD', field: 'pricingType', value: event.target.value })} className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground">
            <option value="free">Free</option>
            <option value="one_time">One-time</option>
            <option value="subscription">Subscription</option>
          </select>
        </div>
        <div className="space-y-2">
          <label htmlFor="agent-price" className="text-sm font-medium">Price</label>
          <Input id="agent-price" type="number" value={state.form.pricingAmount} onChange={(event) => dispatch({ type: 'FIELD', field: 'pricingAmount', value: Number(event.target.value) })} />
        </div>
        <div className="space-y-2">
          <label htmlFor="agent-version" className="text-sm font-medium">Version</label>
          <Input id="agent-version" value={state.form.version} onChange={(event) => dispatch({ type: 'FIELD', field: 'version', value: event.target.value })} />
        </div>
      </div>

      <div className="space-y-2">
        <label htmlFor="agent-changelog" className="text-sm font-medium">Changelog</label>
        <Textarea id="agent-changelog" value={state.form.changelog} onChange={(event) => dispatch({ type: 'FIELD', field: 'changelog', value: event.target.value })} />
      </div>

      {state.error ? (
        <div className="space-y-3 rounded-lg border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive" role="alert">
          <p className="font-medium">{state.error}</p>
          {state.automatedReview ? (
            <ul className="space-y-2">
              {state.automatedReview.findings.map((finding, index) => (
                <li key={`${finding.type}-${finding.field ?? 'field'}-${index}`} className="space-y-1">
                  <div className="flex flex-wrap gap-2">
                    <span className="rounded-md border border-destructive/30 px-2 py-0.5 text-xs font-medium">{finding.type}</span>
                    <span className="rounded-md border border-destructive/30 px-2 py-0.5 text-xs font-medium">{finding.severity}</span>
                    {finding.field ? <span className="rounded-md border border-destructive/30 px-2 py-0.5 text-xs font-medium">{finding.field}</span> : null}
                  </div>
                  <p>{finding.message}</p>
                  {finding.evidence ? <p className="text-xs opacity-80">{finding.evidence}</p> : null}
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}
      {state.success ? <p className="text-sm text-muted-foreground">{state.success}</p> : null}
      <Button type="button" className="min-h-[44px]" disabled={state.loading} onClick={() => void handleSubmit()}>
        Publish Agent
      </Button>
    </div>
  );
}
