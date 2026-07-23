import useSWR, { SWRConfiguration } from 'swr';
import { createHttpClient } from '@/services/http/client';

const client = createHttpClient();

export const fetcher = (url: string) => client.get(url);

export const swrConfig: SWRConfiguration = {
  fetcher,
  revalidateOnFocus: false,
  dedupingInterval: 2000,
};
