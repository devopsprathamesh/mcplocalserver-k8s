import { z } from 'zod';

// Cluster / Namespace
export const ListNamespacesParams = z.object({
  limit: z.number().int().positive().optional(),
});

// Pods
export const ListPodsParams = z.object({
  namespace: z.string().optional(),
  labelSelector: z.string().optional(),
  fieldSelector: z.string().optional(),
  limit: z.number().int().positive().optional(),
});

export const GetPodParams = z.object({
  namespace: z.string(),
  name: z.string(),
});

export const LogsParams = z.object({
  namespace: z.string(),
  name: z.string(),
  container: z.string().optional(),
  tailLines: z.number().int().positive().max(5000).default(200).optional(),
  sinceSeconds: z.number().int().positive().optional(),
  timestamps: z.boolean().optional(),
});

export const ExecParams = z.object({
  namespace: z.string(),
  name: z.string(),
  container: z.string().optional(),
  command: z.array(z.string()).min(1),
  timeoutSeconds: z.number().int().positive().max(3600).optional(),
  dryRun: z.boolean().optional().default(true),
});

// Resources
export const GetResourceParams = z.object({
  group: z.string().optional(),
  version: z.string(),
  kind: z.string(),
  name: z.string().optional(),
  namespace: z.string().optional(),
  labelSelector: z.string().optional(),
  fieldSelector: z.string().optional(),
  limit: z.number().int().positive().optional(),
});

export const ApplyResourceParams = z.object({
  manifestYAML: z.string().min(1),
  serverSideApply: z.boolean().optional().default(true),
  fieldManager: z.string().optional().default('mcp-k8s-server'),
  dryRun: z.boolean().optional().default(true),
});

export const DeleteResourceParams = z.object({
  group: z.string().optional(),
  version: z.string(),
  kind: z.string(),
  name: z.string(),
  namespace: z.string().optional(),
  propagationPolicy: z.enum(['Foreground', 'Background', 'Orphan']).optional(),
  gracePeriodSeconds: z.number().int().nonnegative().optional(),
  dryRun: z.boolean().optional().default(true),
});

// Secrets
export const GetSecretParams = z.object({
  namespace: z.string(),
  name: z.string(),
  keys: z.array(z.string()).optional(),
  showValues: z.boolean().optional(),
});

export const SetSecretParams = z.object({
  namespace: z.string(),
  name: z.string(),
  data: z.record(z.string()).refine((obj) => Object.keys(obj).length > 0, 'data must have at least one key'),
  type: z.string().optional().default('Opaque'),
  base64Encoded: z.boolean().optional().default(false),
  createIfMissing: z.boolean().optional().default(true),
  dryRun: z.boolean().optional().default(true),
});

// Cluster
export const SetContextParams = z.object({
  context: z.string(),
});

export type Infer<T extends z.ZodTypeAny> = z.infer<T>;
