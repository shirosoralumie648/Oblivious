import { useEffect, useMemo, useState } from 'react';

import {
  createPublishingChannelsApi,
  type PublishingChannel,
  type PublishingChannelMessageLog,
  type PublishingChannelStatus,
  type PublishingChannelTestResult,
  type PublishingChannelType,
  type RetryProcessResult
} from '../../features/publishingChannels/publishingChannelsApi';
import { createHttpClient } from '../../services/http/client';

const channelTypes: PublishingChannelType[] = ['webhook', 'feishu', 'wechat', 'discord', 'slack', 'telegram', 'web_embed', 'api'];

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== '') {
    return error.message;
  }
  if (typeof error === 'string' && error.trim() !== '') {
    return error;
  }
  return fallback;
}

function statusLabel(status: PublishingChannelStatus) {
  switch (status) {
    case 'active':
      return 'Active';
    case 'degraded':
      return 'Degraded';
    case 'disabled':
      return 'Disabled';
    default:
      return status;
  }
}

function statusClass(status: PublishingChannelStatus) {
  switch (status) {
    case 'active':
      return 'bg-emerald-50 text-emerald-800';
    case 'degraded':
      return 'bg-amber-50 text-amber-800';
    case 'disabled':
      return 'bg-stone-200 text-stone-700';
    default:
      return 'bg-stone-100 text-stone-700';
  }
}

function endpointLabel(channel: PublishingChannel) {
  const endpoint = channel.config?.allowedOrigin ?? channel.config?.endpointUrl ?? channel.config?.url ?? channel.config?.webhookUrl;
  return typeof endpoint === 'string' && endpoint.trim() !== '' ? endpoint : 'No endpoint configured';
}

function createChannelConfig({
  allowedOrigin,
  botToken,
  channelType,
  embedMode,
  endpointUrl,
  sdkKey,
  sharedSecret,
  signingSecret
}: {
  allowedOrigin: string;
  botToken: string;
  channelType: PublishingChannelType;
  embedMode: 'iframe' | 'web_sdk';
  endpointUrl: string;
  sdkKey: string;
  sharedSecret: string;
  signingSecret: string;
}) {
  const endpoint = endpointUrl.trim();
  const secret = sharedSecret.trim();
  const token = botToken.trim();
  const signing = signingSecret.trim();
  const origin = allowedOrigin.trim();
  const key = sdkKey.trim();

  if (channelType === 'web_embed') {
    return {
      ...(origin ? { allowedOrigin: origin } : {}),
      embedMode,
      ...(key ? { sdkKey: key } : {})
    };
  }

  return {
    ...(endpoint ? { url: endpoint } : {}),
    ...(secret ? { secret } : {}),
    ...(token ? { botToken: token } : {}),
    ...(signing ? { signingSecret: signing } : {})
  };
}

function logStatusClass(status?: string) {
  switch (status) {
    case 'recorded':
      return 'bg-emerald-50 text-emerald-800';
    case 'retry_pending':
    case 'sending':
      return 'bg-amber-50 text-amber-800';
    case 'permanent_failure':
      return 'bg-red-50 text-red-800';
    default:
      return 'bg-stone-100 text-stone-700';
  }
}

function formatLogTime(value?: string) {
  if (!value) {
    return 'No timestamp';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function messagePreview(log: PublishingChannelMessageLog) {
  const transformed = log.transformed_message;
  if (typeof transformed === 'object' && transformed !== null && 'content' in transformed) {
    const content = (transformed as { content?: Array<{ text?: string }> }).content;
    const firstText = content?.find((part) => typeof part.text === 'string' && part.text.trim() !== '')?.text;
    if (firstText) {
      return firstText;
    }
  }
  const raw = log.raw_message;
  if (typeof raw === 'object' && raw !== null && 'text' in raw) {
    const text = (raw as { text?: unknown }).text;
    if (typeof text === 'string' && text.trim() !== '') {
      return text;
    }
  }
  return log.id;
}

function ChannelLogTable({
  emptyText,
  logs
}: {
  emptyText: string;
  logs: PublishingChannelMessageLog[];
}) {
  if (logs.length === 0) {
    return <p className="mt-3 text-sm text-[#625b4f]">{emptyText}</p>;
  }

  return (
    <div className="mt-3 overflow-x-auto">
      <table className="min-w-full border-collapse text-left text-sm">
        <thead className="text-xs uppercase tracking-wide text-[#6d6658]">
          <tr className="border-b border-[#d7d2c4]">
            <th className="py-2 pr-3 font-semibold">Message</th>
            <th className="py-2 pr-3 font-semibold">Direction</th>
            <th className="py-2 pr-3 font-semibold">Status</th>
            <th className="py-2 pr-3 font-semibold">Retries</th>
            <th className="py-2 pr-3 font-semibold">Failure reason</th>
            <th className="py-2 font-semibold">Created</th>
          </tr>
        </thead>
        <tbody>
          {logs.map((log) => (
            <tr className="border-b border-[#eee8dc] last:border-0" key={log.id}>
              <td className="max-w-[220px] py-3 pr-3 align-top">
                <p className="font-medium text-[#181611]">{log.id}</p>
                <p className="mt-1 truncate text-xs text-[#625b4f]">{messagePreview(log)}</p>
              </td>
              <td className="py-3 pr-3 align-top text-[#181611]">{log.direction ?? 'unknown'}</td>
              <td className="py-3 pr-3 align-top">
                <span className={`rounded-full px-2 py-1 text-xs font-semibold ${logStatusClass(log.status)}`}>{log.status}</span>
              </td>
              <td className="py-3 pr-3 align-top text-[#181611]">
                <p>Retries {log.retry_count ?? 0}</p>
                {log.next_retry_at ? <p className="mt-1 text-xs text-[#625b4f]">Next retry: {formatLogTime(log.next_retry_at)}</p> : null}
              </td>
              <td className="max-w-[260px] py-3 pr-3 align-top text-[#625b4f]">{log.failure_reason || log.transform_error || '-'}</td>
              <td className="py-3 align-top text-xs text-[#625b4f]">{formatLogTime(log.created_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function PublishingChannelsPage() {
  const channelsApi = useMemo(() => createPublishingChannelsApi(createHttpClient()), []);
  const [channels, setChannels] = useState<PublishingChannel[]>([]);
  const [channelName, setChannelName] = useState('');
  const [channelType, setChannelType] = useState<PublishingChannelType>('webhook');
  const [conversationID, setConversationID] = useState('');
  const [allowedOrigin, setAllowedOrigin] = useState('');
  const [botToken, setBotToken] = useState('');
  const [embedMode, setEmbedMode] = useState<'iframe' | 'web_sdk'>('iframe');
  const [endpointUrl, setEndpointUrl] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [messageText, setMessageText] = useState('');
  const [sdkKey, setSdkKey] = useState('');
  const [selectedChannelID, setSelectedChannelID] = useState('');
  const [sharedSecret, setSharedSecret] = useState('');
  const [signingSecret, setSigningSecret] = useState('');
  const [actionResults, setActionResults] = useState<Record<string, string>>({});
  const [fallbackChannelIDs, setFallbackChannelIDs] = useState<Record<string, string>>({});
  const [failedMessages, setFailedMessages] = useState<Record<string, PublishingChannelMessageLog[]>>({});
  const [messageLogs, setMessageLogs] = useState<Record<string, PublishingChannelMessageLog[]>>({});
  const [messageLogsLoading, setMessageLogsLoading] = useState(false);
  const [retryLimits, setRetryLimits] = useState<Record<string, string>>({});
  const [retryResults, setRetryResults] = useState<Record<string, RetryProcessResult>>({});
  const [retryingFailedChannelID, setRetryingFailedChannelID] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<string, PublishingChannelTestResult>>({});

  useEffect(() => {
    let cancelled = false;

    const loadChannels = async () => {
      setIsLoading(true);
      setError(null);
      try {
        const nextChannels = await channelsApi.listChannels();
        if (!cancelled) {
          setChannels(nextChannels);
          setSelectedChannelID((current) => current || nextChannels[0]?.id || '');
        }
      } catch (caughtError) {
        if (!cancelled) {
          setError(errorMessage(caughtError, 'Unable to load publishing channels. Retry the request or check the backend session.'));
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadChannels();

    return () => {
      cancelled = true;
    };
  }, [channelsApi]);

  const selectedChannel = channels.find((channel) => channel.id === selectedChannelID) ?? channels[0] ?? null;
  const selectedMessageLogs = selectedChannel ? messageLogs[selectedChannel.id] ?? [] : [];
  const selectedFailedMessages = selectedChannel ? failedMessages[selectedChannel.id] ?? [] : [];
  const selectedRetryLimit = selectedChannel ? retryLimits[selectedChannel.id] ?? '' : '';
  const selectedFallbackChannelID = selectedChannel ? fallbackChannelIDs[selectedChannel.id] ?? '' : '';
  const fallbackChannels = selectedChannel ? channels.filter((channel) => channel.id !== selectedChannel.id) : [];

  useEffect(() => {
    if (!selectedChannel?.id) {
      return;
    }
    let cancelled = false;

    const loadChannelMessageVisibility = async () => {
      setMessageLogsLoading(true);
      setError(null);
      try {
        const [logs, failures] = await Promise.all([
          channelsApi.listChannelMessages(selectedChannel.id),
          channelsApi.listFailedChannelMessages(selectedChannel.id)
        ]);
        if (!cancelled) {
          setMessageLogs((current) => ({ ...current, [selectedChannel.id]: logs }));
          setFailedMessages((current) => ({ ...current, [selectedChannel.id]: failures }));
        }
      } catch (caughtError) {
        if (!cancelled) {
          setError(errorMessage(caughtError, 'Unable to load publishing channel message logs. Retry the request or inspect channel storage.'));
        }
      } finally {
        if (!cancelled) {
          setMessageLogsLoading(false);
        }
      }
    };

    void loadChannelMessageVisibility();

    return () => {
      cancelled = true;
    };
  }, [channelsApi, selectedChannel?.id]);

  const createChannel = async () => {
    const name = channelName.trim();
    if (name === '') {
      return;
    }

    setIsCreating(true);
    setError(null);
    try {
      const created = await channelsApi.createChannel({
        config: createChannelConfig({
          allowedOrigin,
          botToken,
          channelType,
          embedMode,
          endpointUrl,
          sdkKey,
          sharedSecret,
          signingSecret
        }),
        name,
        type: channelType
      });
      setChannels((current) => [created, ...current.filter((channel) => channel.id !== created.id)]);
      setSelectedChannelID(created.id);
      setChannelName('');
      setChannelType('webhook');
      setAllowedOrigin('');
      setBotToken('');
      setEmbedMode('iframe');
      setEndpointUrl('');
      setSdkKey('');
      setSharedSecret('');
      setSigningSecret('');
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to create publishing channel. Retry the request or check the backend session.'));
    } finally {
      setIsCreating(false);
    }
  };

  const testChannel = async (channel: PublishingChannel) => {
    setError(null);
    try {
      const result = await channelsApi.testChannel(channel.id);
      setTestResults((current) => ({ ...current, [channel.id]: result }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to test publishing channel. Retry the request or check the channel config.'));
    }
  };

  const switchStatus = async (channel: PublishingChannel, status: PublishingChannelStatus) => {
    setError(null);
    try {
      const updated = await channelsApi.updateChannelStatus(channel, status);
      setChannels((current) => current.map((item) => (item.id === updated.id ? updated : item)));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to update publishing channel status. Retry the request or check permissions.'));
    }
  };

  const sendMessage = async () => {
    if (!selectedChannel) {
      return;
    }
    const trimmedConversationID = conversationID.trim();
    const trimmedMessage = messageText.trim();
    if (trimmedConversationID === '' || trimmedMessage === '') {
      return;
    }

    setError(null);
    try {
      const log = await channelsApi.sendChannelMessage(selectedChannel.id, {
        conversation_id: trimmedConversationID,
        role: 'assistant',
        text: trimmedMessage
      });
      setActionResults((current) => ({ ...current, [selectedChannel.id]: `Last send: ${log.status}` }));
      const [logs, failures] = await Promise.all([
        channelsApi.listChannelMessages(selectedChannel.id),
        channelsApi.listFailedChannelMessages(selectedChannel.id)
      ]);
      setMessageLogs((current) => ({ ...current, [selectedChannel.id]: logs }));
      setFailedMessages((current) => ({ ...current, [selectedChannel.id]: failures }));
      setConversationID('');
      setMessageText('');
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to send publishing channel message. Retry the request or inspect channel health.'));
    }
  };

  const retryFailedMessages = async () => {
    if (!selectedChannel) {
      return;
    }
    const fallbackChannelID = selectedFallbackChannelID.trim();
    const parsedLimit = Number.parseInt(selectedRetryLimit, 10);
    const payload = {
      ...(fallbackChannelID ? { fallback_channel_id: fallbackChannelID } : {}),
      ...(fallbackChannelID ? { force: true } : {}),
      ...(Number.isFinite(parsedLimit) && parsedLimit > 0 ? { limit: parsedLimit } : {})
    };

    setRetryingFailedChannelID(selectedChannel.id);
    setError(null);
    try {
      const result = await channelsApi.retryFailedChannelMessages(selectedChannel.id, payload);
      const [logs, failures] = await Promise.all([
        channelsApi.listChannelMessages(selectedChannel.id),
        channelsApi.listFailedChannelMessages(selectedChannel.id)
      ]);
      setRetryResults((current) => ({ ...current, [selectedChannel.id]: result }));
      setMessageLogs((current) => ({ ...current, [selectedChannel.id]: logs }));
      setFailedMessages((current) => ({ ...current, [selectedChannel.id]: failures }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to retry failed publishing channel messages. Retry the request or inspect channel health.'));
    } finally {
      setRetryingFailedChannelID(null);
    }
  };

  return (
    <section className="mx-auto max-w-6xl space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">External publishing</p>
        <h1 className="font-heading text-3xl font-semibold text-[#181611]">Publishing Channels</h1>
        <p className="max-w-3xl text-sm leading-6 text-[#625b4f]">
          Configure webhook and platform adapters, test delivery, and manually recover degraded publishing paths.
        </p>
      </header>

      {error ? (
        <p className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800" role="alert">
          {error}
        </p>
      ) : null}

      <section className="rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] p-5" aria-label="Create publishing channel">
        <h2 className="text-base font-semibold">Create channel</h2>
        <form
          className="mt-4 grid gap-4 md:grid-cols-2 xl:grid-cols-[minmax(0,1fr)_160px_minmax(0,1fr)_minmax(0,1fr)_auto]"
          onSubmit={(event) => {
            event.preventDefault();
            void createChannel();
          }}
        >
          <label className="block text-sm font-medium">
            Channel name
            <input
              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
              onChange={(event) => setChannelName(event.target.value)}
              value={channelName}
            />
          </label>
          <label className="block text-sm font-medium">
            Channel type
            <select
              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
              onChange={(event) => setChannelType(event.target.value as PublishingChannelType)}
              value={channelType}
            >
              {channelTypes.map((type) => (
                <option key={type} value={type}>
                  {type}
                </option>
              ))}
            </select>
          </label>
          {channelType === 'web_embed' ? (
            <>
              <label className="block text-sm font-medium">
                Allowed origin
                <input
                  className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                  onChange={(event) => setAllowedOrigin(event.target.value)}
                  placeholder="https://app.example"
                  value={allowedOrigin}
                />
              </label>
              <label className="block text-sm font-medium">
                SDK key
                <input
                  className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                  onChange={(event) => setSdkKey(event.target.value)}
                  value={sdkKey}
                />
              </label>
              <fieldset className="block text-sm font-medium">
                Embed mode
                <div className="mt-2 flex rounded-lg border border-[#d7d2c4] bg-white p-1">
                  {[
                    ['iframe', 'iframe'],
                    ['web_sdk', 'Web SDK']
                  ].map(([value, label]) => (
                    <label className="flex-1" key={value}>
                      <input
                        checked={embedMode === value}
                        className="sr-only"
                        onChange={() => setEmbedMode(value as 'iframe' | 'web_sdk')}
                        type="radio"
                        value={value}
                      />
                      <span className={`block rounded-md px-3 py-1.5 text-center text-xs font-semibold ${embedMode === value ? 'bg-[#181611] text-white' : 'text-[#625b4f]'}`}>
                        {label}
                      </span>
                    </label>
                  ))}
                </div>
              </fieldset>
            </>
          ) : (
            <>
              <label className="block text-sm font-medium">
                Endpoint URL
                <input
                  className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                  onChange={(event) => setEndpointUrl(event.target.value)}
                  placeholder="https://hooks.example/channel"
                  value={endpointUrl}
                />
              </label>
              <label className="block text-sm font-medium">
                Shared secret
                <input
                  className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                  onChange={(event) => setSharedSecret(event.target.value)}
                  type="password"
                  value={sharedSecret}
                />
              </label>
              {channelType === 'slack' || channelType === 'telegram' ? (
                <label className="block text-sm font-medium">
                  Bot token
                  <input
                    className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                    onChange={(event) => setBotToken(event.target.value)}
                    type="password"
                    value={botToken}
                  />
                </label>
              ) : null}
              {channelType === 'slack' ? (
                <label className="block text-sm font-medium">
                  Signing secret
                  <input
                    className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                    onChange={(event) => setSigningSecret(event.target.value)}
                    type="password"
                    value={signingSecret}
                  />
                </label>
              ) : null}
            </>
          )}
          <button
            className="self-end rounded-lg bg-[#181611] px-4 py-2 text-sm font-semibold text-white disabled:opacity-60 md:col-span-2 xl:col-span-1"
            disabled={isCreating || channelName.trim() === ''}
            type="submit"
          >
            {isCreating ? 'Creating...' : 'Create channel'}
          </button>
        </form>
      </section>

      <section className="rounded-lg border border-[#d7d2c4] bg-white p-5" aria-label="Publishing channel send test">
        <h2 className="text-base font-semibold">Manual delivery test</h2>
        <form
          className="mt-4 grid gap-4 md:grid-cols-[220px_minmax(0,1fr)_minmax(0,1fr)_auto]"
          onSubmit={(event) => {
            event.preventDefault();
            void sendMessage();
          }}
        >
          <label className="block text-sm font-medium">
            Channel
            <select
              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
              onChange={(event) => setSelectedChannelID(event.target.value)}
              value={selectedChannel?.id ?? ''}
            >
              {channels.map((channel) => (
                <option key={channel.id} value={channel.id}>
                  {channel.name}
                </option>
              ))}
            </select>
          </label>
          <label className="block text-sm font-medium">
            Conversation ID
            <input
              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
              onChange={(event) => setConversationID(event.target.value)}
              value={conversationID}
            />
          </label>
          <label className="block text-sm font-medium">
            Message text
            <input
              className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
              onChange={(event) => setMessageText(event.target.value)}
              value={messageText}
            />
          </label>
          <button
            className="self-end rounded-lg border border-[#181611] px-4 py-2 text-sm font-semibold text-[#181611] disabled:opacity-60"
            disabled={!selectedChannel || conversationID.trim() === '' || messageText.trim() === ''}
            type="submit"
          >
            Send message
          </button>
        </form>
      </section>

      {selectedChannel ? (
        <section className="rounded-lg border border-[#d7d2c4] bg-white p-5" aria-label="Publishing channel message visibility">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold">Message visibility</h2>
              <p className="mt-1 text-sm text-[#625b4f]">{selectedChannel.name}</p>
            </div>
            <span className="rounded-full bg-[#eee8dc] px-3 py-1 text-xs font-semibold text-[#625b4f]">
              {messageLogsLoading ? 'Loading logs' : `${selectedMessageLogs.length} recent / ${selectedFailedMessages.length} failed`}
            </span>
          </div>
          <div className="mt-5 grid gap-5 xl:grid-cols-2">
            <div>
              <h3 className="text-sm font-semibold text-[#181611]">Recent messages</h3>
              <ChannelLogTable emptyText="No recent messages recorded for this channel." logs={selectedMessageLogs} />
            </div>
            <div>
              <div className="flex flex-wrap items-end justify-between gap-3">
                <h3 className="text-sm font-semibold text-[#181611]">Failed retry queue</h3>
                <form
                  aria-label="Failed retry queue controls"
                  className="grid gap-3 text-sm sm:grid-cols-[minmax(0,180px)_110px_auto]"
                  onSubmit={(event) => {
                    event.preventDefault();
                    void retryFailedMessages();
                  }}
                >
                  <label className="block font-medium">
                    Fallback channel
                    <select
                      className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                      onChange={(event) =>
                        selectedChannel
                          ? setFallbackChannelIDs((current) => ({ ...current, [selectedChannel.id]: event.target.value }))
                          : undefined
                      }
                      value={selectedFallbackChannelID}
                    >
                      <option value="">None</option>
                      {fallbackChannels.map((channel) => (
                        <option key={channel.id} value={channel.id}>
                          {channel.name}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="block font-medium">
                    Retry limit
                    <input
                      className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                      min="1"
                      onChange={(event) =>
                        selectedChannel
                          ? setRetryLimits((current) => ({ ...current, [selectedChannel.id]: event.target.value }))
                          : undefined
                      }
                      type="number"
                      value={selectedRetryLimit}
                    />
                  </label>
                  <button
                    className="self-end rounded-lg border border-[#181611] px-3 py-2 text-sm font-semibold text-[#181611] disabled:opacity-60"
                    disabled={!selectedChannel || retryingFailedChannelID === selectedChannel.id}
                    type="submit"
                  >
                    {retryingFailedChannelID === selectedChannel.id
                      ? 'Retrying...'
                      : selectedFallbackChannelID.trim() !== ''
                        ? 'Switch queue to fallback'
                        : 'Retry failed messages'}
                  </button>
                </form>
              </div>
              {retryResults[selectedChannel.id] ? (
                <p className="mt-3 text-sm text-[#181611]">
                  Retry result: claimed {retryResults[selectedChannel.id].claimed}, succeeded {retryResults[selectedChannel.id].succeeded}, failed{' '}
                  {retryResults[selectedChannel.id].failed}, permanent failures {retryResults[selectedChannel.id].permanentFailures}
                </p>
              ) : null}
              <ChannelLogTable emptyText="No failed retry messages waiting on this channel." logs={selectedFailedMessages} />
            </div>
          </div>
        </section>
      ) : null}

      <section className="rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] p-5">
        <div className="flex items-center justify-between gap-4">
          <h2 className="text-base font-semibold">Configured channels</h2>
          <span className="text-xs text-[#6d6658]">{channels.length} total</span>
        </div>
        {isLoading ? <p className="mt-4 text-sm text-[#625b4f]">Loading publishing channels...</p> : null}
        {!isLoading && channels.length === 0 ? <p className="mt-4 text-sm text-[#625b4f]">No publishing channels configured.</p> : null}
        {channels.length > 0 ? (
          <ul aria-label="Publishing channel list" className="mt-4 grid gap-3">
            {channels.map((channel) => (
              <li key={channel.id} className="rounded-lg border border-[#d7d2c4] bg-white p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-base font-semibold text-[#181611]">{channel.name}</h3>
                      <span className="rounded-full bg-[#eee8dc] px-2 py-1 text-xs font-medium text-[#625b4f]">{channel.type}</span>
                      <span className={`rounded-full px-2 py-1 text-xs font-semibold ${statusClass(channel.status)}`}>{statusLabel(channel.status)}</span>
                    </div>
                    <p className="mt-2 text-sm text-[#625b4f]">{endpointLabel(channel)}</p>
                    {testResults[channel.id] ? <p className="mt-2 text-sm text-emerald-700">{testResults[channel.id].message}</p> : null}
                    {actionResults[channel.id] ? <p className="mt-2 text-sm text-[#181611]">{actionResults[channel.id]}</p> : null}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <button
                      className="rounded-lg border border-[#d7d2c4] px-3 py-2 text-sm font-medium"
                      onClick={() => void testChannel(channel)}
                      type="button"
                    >
                      Test channel {channel.name}
                    </button>
                    {channel.status !== 'active' ? (
                      <button
                        className="rounded-lg bg-emerald-700 px-3 py-2 text-sm font-semibold text-white"
                        onClick={() => void switchStatus(channel, 'active')}
                        type="button"
                      >
                        Activate {channel.name}
                      </button>
                    ) : (
                      <button
                        className="rounded-lg border border-[#d7d2c4] px-3 py-2 text-sm font-medium"
                        onClick={() => void switchStatus(channel, 'disabled')}
                        type="button"
                      >
                        Disable {channel.name}
                      </button>
                    )}
                  </div>
                </div>
              </li>
            ))}
          </ul>
        ) : null}
      </section>
    </section>
  );
}
