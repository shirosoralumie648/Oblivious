import { z } from 'zod';

export const userPreferencesSchema = z.object({
  defaultMode: z.enum(['chat', 'solo']),
  modelStrategy: z.string(),
  networkEnabledHint: z.boolean(),
});

export const loginFormSchema = z.object({
  email: z.string().email('无效邮箱'),
  password: z.string().min(6, '密码至少6位'),
});

export const channelFormSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  provider: z.string().min(1, 'Provider is required'),
  apiKey: z.string(),
  baseURL: z.string().min(1, 'Base URL is required'),
  models: z.string(),
  groups: z.string(),
  rpmLimit: z.string(),
  tpmLimit: z.string(),
  priority: z.string(),
  estimatedCostPer1K: z.string(),
  costMultiplier: z.string(),
  weight: z.string(),
});
