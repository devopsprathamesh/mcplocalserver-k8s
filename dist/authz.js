import { z } from 'zod';
export const MutatingOptionsSchema = z.object({
    dryRun: z.boolean().optional().default(true),
});
const parseCsv = (value) => (value || '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
export function isReadOnly() {
    return process.env.MCP_K8S_READONLY === 'true';
}
export function isNamespaceAllowed(namespace) {
    if (!namespace)
        return true; // allow cluster-scoped if namespace not applicable
    const allow = parseCsv(process.env.MCP_K8S_NAMESPACE_ALLOWLIST);
    if (allow.length === 0)
        return true; // no allowlist set => allow
    return allow.includes(namespace);
}
export function isKindAllowed(kind) {
    if (!kind)
        return true;
    const allow = parseCsv(process.env.MCP_K8S_KIND_ALLOWLIST);
    if (allow.length === 0)
        return true;
    return allow.includes(kind);
}
export class McpGuardError extends Error {
    code;
    details;
    constructor(message, code = 'GUARD_VIOLATION', details) {
        super(message);
        this.code = code;
        this.details = details;
    }
}
export function enforceMutatingGuards(logger, toolName, options) {
    if (isReadOnly()) {
        throw new McpGuardError(`${toolName} is blocked in read-only mode`, 'READ_ONLY_BLOCKED', {
            suggestion: 'Unset MCP_K8S_READONLY or use dryRun only',
        });
    }
    if (!isNamespaceAllowed(options.namespace)) {
        throw new McpGuardError(`Namespace ${options.namespace} is not in allowlist`, 'NS_NOT_ALLOWED', {
            suggestion: 'Add namespace to MCP_K8S_NAMESPACE_ALLOWLIST',
        });
    }
    if (!isKindAllowed(options.kind)) {
        throw new McpGuardError(`Kind ${options.kind} is not in allowlist`, 'KIND_NOT_ALLOWED', {
            suggestion: 'Add kind to MCP_K8S_KIND_ALLOWLIST',
        });
    }
}
// Simple token-bucket per-tool rate limiter
class TokenBucket {
    capacity;
    refillPerSecond;
    tokens;
    lastRefill;
    constructor(capacity, refillPerSecond) {
        this.capacity = capacity;
        this.refillPerSecond = refillPerSecond;
        this.tokens = capacity;
        this.lastRefill = Date.now();
    }
    tryRemoveToken() {
        const now = Date.now();
        const elapsed = (now - this.lastRefill) / 1000;
        const refill = Math.floor(elapsed * this.refillPerSecond);
        if (refill > 0) {
            this.tokens = Math.min(this.capacity, this.tokens + refill);
            this.lastRefill = now;
        }
        if (this.tokens > 0) {
            this.tokens -= 1;
            return true;
        }
        return false;
    }
}
const buckets = new Map();
export function rateLimit(toolName, burst = 10, rate = 5) {
    const key = toolName;
    let bucket = buckets.get(key);
    if (!bucket) {
        bucket = new TokenBucket(burst, rate);
        buckets.set(key, bucket);
    }
    if (!bucket.tryRemoveToken()) {
        throw new McpGuardError('Rate limit exceeded', 'RATE_LIMIT', {
            suggestion: 'Slow down or try again shortly',
        });
    }
}
export function redactSecret(value) {
    if (!value)
        return 'REDACTED';
    return 'REDACTED';
}
