import { type FormEvent, useEffect, useMemo, useState } from 'react';

import { createConsoleApi, type BillingCheckoutProvider, type ConsoleBillingInvoiceSummary, type ConsoleBillingSummary } from '../../features/console/api';
import { ConsoleWorkbenchLayout } from '../../features/console/components/ConsoleWorkbenchLayout';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary } from '../../types/api';

export function BillingPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [billingSummary, setBillingSummary] = useState<ConsoleBillingSummary | null>(null);
  const [invoices, setInvoices] = useState<ConsoleBillingInvoiceSummary[]>([]);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [checkoutError, setCheckoutError] = useState<string | null>(null);
  const [checkoutUrl, setCheckoutUrl] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isStartingCheckout, setIsStartingCheckout] = useState(false);
  const [topUpAmount, setTopUpAmount] = useState('25');
  const [topUpProvider, setTopUpProvider] = useState<BillingCheckoutProvider>('stripe');
  const paymentProviders = useMemo(
    () => normalizedBillingCheckoutProviders(billingSummary?.paymentProviders),
    [billingSummary?.paymentProviders]
  );
  const hasPaymentProviders = paymentProviders.length > 0;

  useEffect(() => {
    let cancelled = false;

    const loadBilling = async () => {
      const [access, billing, invoiceList] = await Promise.allSettled([
        consoleApi.getAccess(),
        consoleApi.getBilling(),
        consoleApi.listInvoices()
      ]);

      if (cancelled) {
        return;
      }

      setAccessSummary(access.status === 'fulfilled' ? access.value : null);
      if (billing.status === 'fulfilled') {
        setBillingSummary(billing.value);
        setErrorMessage(null);
      } else {
        setBillingSummary(null);
        setErrorMessage('Unable to load billing summary.');
      }
      setInvoices(invoiceList.status === 'fulfilled' ? invoiceList.value : []);
      setIsLoading(false);
    };

    void loadBilling();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  useEffect(() => {
    if (paymentProviders.length === 0) {
      return;
    }
    if (!paymentProviders.includes(topUpProvider)) {
      setTopUpProvider(paymentProviders[0]);
    }
  }, [paymentProviders, topUpProvider]);

  const handleTopUpCheckout = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const amount = Number(topUpAmount);

    if (!Number.isFinite(amount) || amount <= 0) {
      setCheckoutError('Enter a positive top-up amount.');
      setCheckoutUrl(null);
      return;
    }
    if (!hasPaymentProviders) {
      setCheckoutError('No payment provider is configured for checkout.');
      setCheckoutUrl(null);
      return;
    }

    setCheckoutError(null);
    setCheckoutUrl(null);
    setIsStartingCheckout(true);
    try {
      const checkout = await consoleApi.createBillingCheckout({
        amount,
        kind: 'topup',
        provider: topUpProvider
      });
      setCheckoutUrl(checkout.url);
    } catch {
      setCheckoutError('Unable to start top-up checkout.');
    } finally {
      setIsStartingCheckout(false);
    }
  };

  return (
    <ConsoleWorkbenchLayout
      accessSummary={accessSummary}
      description="Review current workspace cost and billing activity."
      errorMessage={errorMessage}
      siblingLinks={[{ label: 'Open usage', to: '/console/usage' }]}
      title="Billing"
    >
      {isLoading ? (
        <p>Loading billing summary…</p>
      ) : billingSummary ? (
        <>
          <p>{`Requests: ${billingSummary.requests}`}</p>
          <p>{`Input tokens: ${billingSummary.inputTokens}`}</p>
          <p>{`Output tokens: ${billingSummary.outputTokens}`}</p>
          <p>{`Estimated cost: $${billingSummary.estimatedCostUsd.toFixed(4)}`}</p>
          <p>{`Balance: $${billingSummary.balanceUsd.toFixed(2)}`}</p>
          <p>{`Credit limit: $${billingSummary.creditLimitUsd.toFixed(2)}`}</p>
          <p>{`Current spend: $${billingSummary.currentSpendUsd.toFixed(4)}`}</p>
          <section aria-label="Quota top-up checkout">
            <h2>Add balance</h2>
            <form onSubmit={handleTopUpCheckout}>
              <label htmlFor="billing-top-up-amount">Top-up amount USD</label>
              <input
                id="billing-top-up-amount"
                min="1"
                onChange={(event) => setTopUpAmount(event.target.value)}
                step="0.01"
                type="number"
                value={topUpAmount}
              />
              <label htmlFor="billing-top-up-provider">Payment provider</label>
              <select
                id="billing-top-up-provider"
                onChange={(event) => setTopUpProvider(event.target.value as BillingCheckoutProvider)}
                disabled={!hasPaymentProviders}
                value={topUpProvider}
              >
                {paymentProviders.map((provider) => (
                  <option key={provider} value={provider}>
                    {billingCheckoutProviderLabel(provider)}
                  </option>
                ))}
              </select>
              <button disabled={isStartingCheckout || !hasPaymentProviders} type="submit">
                {isStartingCheckout ? 'Starting checkout…' : 'Start top-up checkout'}
              </button>
            </form>
            {checkoutError ? <p role="alert">{checkoutError}</p> : null}
            {checkoutUrl ? <a href={checkoutUrl}>{checkoutLinkLabel(topUpProvider)}</a> : null}
          </section>
          {billingSummary.nextInvoice ? (
            <p>{`Next invoice: ${billingSummary.nextInvoice.status} - $${billingSummary.nextInvoice.amountUsd.toFixed(4)} - due ${formatInvoiceDueDate(billingSummary.nextInvoice.dueAt)}`}</p>
          ) : null}
          <section aria-label="Invoice history">
            <h2>Invoice history</h2>
            {invoices.length > 0 ? (
              <table>
                <thead>
                  <tr>
                    <th scope="col">Invoice ID</th>
                    <th scope="col">Status</th>
                    <th scope="col">Amount</th>
                    <th scope="col">Due date</th>
                  </tr>
                </thead>
                <tbody>
                  {invoices.map((invoice) => (
                    <tr key={invoice.id}>
                      <td>{invoice.id}</td>
                      <td>{invoice.status}</td>
                      <td>{`$${invoice.amountUsd.toFixed(4)}`}</td>
                      <td>{formatInvoiceDueDate(invoice.dueAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <p>No invoices available.</p>
            )}
          </section>
        </>
      ) : (
        <p>Billing summary unavailable.</p>
      )}
    </ConsoleWorkbenchLayout>
  );
}

function normalizedBillingCheckoutProviders(
  providers: ConsoleBillingSummary['paymentProviders'] | undefined
): BillingCheckoutProvider[] {
  return (providers ?? [])
    .map((provider) => provider.name.trim().toLowerCase())
    .filter((provider, index, values) => provider !== '' && values.indexOf(provider) === index);
}

function billingCheckoutProviderLabel(provider: BillingCheckoutProvider) {
  switch (provider) {
    case 'alipay':
      return 'Alipay';
    case 'wechatpay':
      return 'WeChat Pay';
    case 'stripe':
      return 'Stripe';
    default:
      return provider;
  }
}

function checkoutLinkLabel(provider: BillingCheckoutProvider) {
  return provider === 'stripe' ? 'Continue to checkout' : `Continue ${billingCheckoutProviderLabel(provider)} checkout`;
}

function formatInvoiceDueDate(value: string) {
  return new Intl.DateTimeFormat('en-US', {
    day: 'numeric',
    month: 'short',
    timeZone: 'UTC',
    year: 'numeric'
  }).format(new Date(value));
}
