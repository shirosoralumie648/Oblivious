import { type FormEvent, useEffect, useMemo, useState } from 'react';

import { createConsoleApi, type BillingCheckoutProvider, type ConsoleBillingInvoiceSummary, type ConsoleBillingSummary } from '../../features/console/api';
import { ConsoleWorkbenchLayout } from '../../features/console/components/ConsoleWorkbenchLayout';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary, PackageOption } from '../../types/api';

export function BillingPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [billingSummary, setBillingSummary] = useState<ConsoleBillingSummary | null>(null);
  const [invoices, setInvoices] = useState<ConsoleBillingInvoiceSummary[]>([]);
  const [packages, setPackages] = useState<PackageOption[]>([]);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [topUpCheckoutError, setTopUpCheckoutError] = useState<string | null>(null);
  const [topUpCheckoutUrl, setTopUpCheckoutUrl] = useState<string | null>(null);
  const [subscriptionCheckoutError, setSubscriptionCheckoutError] = useState<string | null>(null);
  const [subscriptionCheckoutUrl, setSubscriptionCheckoutUrl] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isStartingTopUpCheckout, setIsStartingTopUpCheckout] = useState(false);
  const [isStartingSubscriptionCheckout, setIsStartingSubscriptionCheckout] = useState(false);
  const [topUpAmount, setTopUpAmount] = useState('25');
  const [topUpProvider, setTopUpProvider] = useState<BillingCheckoutProvider>('stripe');
  const [subscriptionProvider, setSubscriptionProvider] = useState<BillingCheckoutProvider>('stripe');
  const [selectedPackageId, setSelectedPackageId] = useState('');
  const paymentProviders = useMemo(
    () => normalizedBillingCheckoutProviders(billingSummary?.paymentProviders),
    [billingSummary?.paymentProviders]
  );
  const hasPaymentProviders = paymentProviders.length > 0;
  const selectedPackage = useMemo(
    () => packages.find((packageOption) => packageOption.id === selectedPackageId) ?? packages[0] ?? null,
    [packages, selectedPackageId]
  );

  useEffect(() => {
    let cancelled = false;

    const loadBilling = async () => {
      const [access, billing, invoiceList, packageList] = await Promise.allSettled([
        consoleApi.getAccess(),
        consoleApi.getBilling(),
        consoleApi.listInvoices(),
        consoleApi.listPackages()
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
      setPackages(packageList.status === 'fulfilled' && Array.isArray(packageList.value) ? packageList.value : []);
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
    if (!paymentProviders.includes(subscriptionProvider)) {
      setSubscriptionProvider(paymentProviders[0]);
    }
  }, [paymentProviders, subscriptionProvider, topUpProvider]);

  useEffect(() => {
    if (packages.length === 0) {
      if (selectedPackageId !== '') {
        setSelectedPackageId('');
      }
      return;
    }
    if (!packages.some((packageOption) => packageOption.id === selectedPackageId)) {
      setSelectedPackageId(packages[0].id);
    }
  }, [packages, selectedPackageId]);

  const handleTopUpCheckout = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const amount = Number(topUpAmount);

    if (!Number.isFinite(amount) || amount <= 0) {
      setTopUpCheckoutError('Enter a positive top-up amount.');
      setTopUpCheckoutUrl(null);
      return;
    }
    if (!hasPaymentProviders) {
      setTopUpCheckoutError('No payment provider is configured for checkout.');
      setTopUpCheckoutUrl(null);
      return;
    }

    setTopUpCheckoutError(null);
    setTopUpCheckoutUrl(null);
    setIsStartingTopUpCheckout(true);
    try {
      const checkout = await consoleApi.createBillingCheckout({
        amount,
        kind: 'topup',
        provider: topUpProvider
      });
      setTopUpCheckoutUrl(checkout.url);
    } catch {
      setTopUpCheckoutError('Unable to start top-up checkout.');
    } finally {
      setIsStartingTopUpCheckout(false);
    }
  };

  const handleSubscriptionCheckout = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!selectedPackage) {
      setSubscriptionCheckoutError('No subscription package is available.');
      setSubscriptionCheckoutUrl(null);
      return;
    }
    if (!hasPaymentProviders) {
      setSubscriptionCheckoutError('No payment provider is configured for checkout.');
      setSubscriptionCheckoutUrl(null);
      return;
    }

    setSubscriptionCheckoutError(null);
    setSubscriptionCheckoutUrl(null);
    setIsStartingSubscriptionCheckout(true);
    try {
      const checkout = await consoleApi.createBillingCheckout({
        kind: 'subscription',
        packageId: selectedPackage.id,
        provider: subscriptionProvider
      });
      setSubscriptionCheckoutUrl(checkout.url);
    } catch {
      setSubscriptionCheckoutError('Unable to start subscription checkout.');
    } finally {
      setIsStartingSubscriptionCheckout(false);
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
          <section aria-label="Subscription packages">
            <h2>Subscription packages</h2>
            {packages.length > 0 && selectedPackage ? (
              <form onSubmit={handleSubscriptionCheckout}>
                <label htmlFor="billing-subscription-package">Package</label>
                <select
                  id="billing-subscription-package"
                  onChange={(event) => setSelectedPackageId(event.target.value)}
                  value={selectedPackage.id}
                >
                  {packages.map((packageOption) => (
                    <option key={packageOption.id} value={packageOption.id}>
                      {formatPackageOptionLabel(packageOption)}
                    </option>
                  ))}
                </select>
                <p>{`Quota credit: $${selectedPackage.quotaAmount.toFixed(2)}`}</p>
                <p>{`Token quota: ${formatInteger(selectedPackage.tokenQuota)}`}</p>
                <p>{`Agent limit: ${formatInteger(selectedPackage.agentLimit)}`}</p>
                <p>{`Max tokens per request: ${formatInteger(selectedPackage.maxTokensPerRequest)}`}</p>
                <label htmlFor="billing-subscription-provider">Subscription payment provider</label>
                <select
                  id="billing-subscription-provider"
                  onChange={(event) => setSubscriptionProvider(event.target.value as BillingCheckoutProvider)}
                  disabled={!hasPaymentProviders}
                  value={subscriptionProvider}
                >
                  {paymentProviders.map((provider) => (
                    <option key={provider} value={provider}>
                      {billingCheckoutProviderLabel(provider)}
                    </option>
                  ))}
                </select>
                <button disabled={isStartingSubscriptionCheckout || !hasPaymentProviders} type="submit">
                  {isStartingSubscriptionCheckout ? 'Starting checkout…' : 'Start subscription checkout'}
                </button>
              </form>
            ) : (
              <p>No subscription packages available.</p>
            )}
            {subscriptionCheckoutError ? <p role="alert">{subscriptionCheckoutError}</p> : null}
            {subscriptionCheckoutUrl ? <a href={subscriptionCheckoutUrl}>{checkoutLinkLabel(subscriptionProvider)}</a> : null}
          </section>
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
              <button disabled={isStartingTopUpCheckout || !hasPaymentProviders} type="submit">
                {isStartingTopUpCheckout ? 'Starting checkout…' : 'Start top-up checkout'}
              </button>
            </form>
            {topUpCheckoutError ? <p role="alert">{topUpCheckoutError}</p> : null}
            {topUpCheckoutUrl ? <a href={topUpCheckoutUrl}>{checkoutLinkLabel(topUpProvider)}</a> : null}
          </section>
          {billingSummary.nextInvoice ? (
            <p>{`Next invoice: ${billingSummary.nextInvoice.status} - $${billingSummary.nextInvoice.amountUsd.toFixed(4)} - due ${formatInvoiceDueDate(billingSummary.nextInvoice.dueAt)}`}</p>
          ) : null}
          <section aria-label="Invoice history">
            <h2>Invoice history</h2>
            {invoices.length > 0 ? (
              <div className="min-w-0 max-w-full overflow-x-auto">
                <table className="min-w-[640px] border-collapse text-left text-sm">
                  <thead>
                    <tr>
                      <th scope="col">Invoice ID</th>
                      <th scope="col">Status</th>
                      <th scope="col">Amount</th>
                      <th scope="col">Due date</th>
                      <th scope="col">Documents</th>
                    </tr>
                  </thead>
                  <tbody>
                    {invoices.map((invoice) => (
                      <tr key={invoice.id}>
                        <td>{invoice.id}</td>
                        <td>{invoice.status}</td>
                        <td>{`$${invoice.amountUsd.toFixed(4)}`}</td>
                        <td>{formatInvoiceDueDate(invoice.dueAt)}</td>
                        <td>
                          {invoice.hostedInvoiceUrl || invoice.invoicePdf ? (
                            <>
                              {invoice.hostedInvoiceUrl ? <a href={invoice.hostedInvoiceUrl}>View invoice</a> : null}
                              {invoice.hostedInvoiceUrl && invoice.invoicePdf ? ' ' : null}
                              {invoice.invoicePdf ? <a href={invoice.invoicePdf}>Download PDF</a> : null}
                            </>
                          ) : (
                            '-'
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
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

function formatPackageOptionLabel(packageOption: PackageOption) {
  return `${packageOption.name} - $${packageOption.price.toFixed(2)} - ${formatPackageDuration(packageOption.durationDays)}`;
}

function formatPackageDuration(durationDays: number | undefined) {
  return durationDays && durationDays > 0 ? `${durationDays} days` : 'ongoing';
}

// Optimization: Reusing Intl formatter instances reduces expensive instantiation overhead on every render.
// Measurement: Reduces CPU time when rendering the billing summary and invoice history.
const integerFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 });
const invoiceDueDateFormatter = new Intl.DateTimeFormat('en-US', {
  day: 'numeric',
  month: 'short',
  timeZone: 'UTC',
  year: 'numeric'
});

function formatInteger(value: number) {
  return integerFormatter.format(value);
}

function formatInvoiceDueDate(value: string) {
  return invoiceDueDateFormatter.format(new Date(value));
}
