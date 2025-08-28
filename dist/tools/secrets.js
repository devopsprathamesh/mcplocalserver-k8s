import { getClients } from '../k8s.js';
import { GetSecretParams, SetSecretParams } from '../schemas.js';
import { enforceMutatingGuards, rateLimit, redactSecret } from '../authz.js';
export function registerSecretTools(server, logger) {
    const clients = getClients(logger);
    server.tool('secrets.get', 'Get a secret (redacted by default)', GetSecretParams.shape, async (args) => {
        rateLimit('secrets.get');
        const body = await clients.coreV1.readNamespacedSecret({ name: args.name, namespace: args.namespace });
        const keys = args.keys && args.keys.length > 0 ? args.keys : Object.keys(body.data || {});
        const showValues = args.showValues === true && process.env.MCP_K8S_READONLY !== 'true';
        const data = {};
        for (const k of keys) {
            const v = body.data?.[k];
            if (showValues && typeof v === 'string')
                data[k] = v; // base64 values as-is
            else
                data[k] = redactSecret(v);
        }
        return { content: [{ type: 'text', text: JSON.stringify({ type: body.type, data }) }] };
    });
    server.tool('secrets.set', 'Create/update a secret with provided keys (values never logged)', SetSecretParams.shape, async (args) => {
        rateLimit('secrets.set');
        enforceMutatingGuards(logger, 'secrets.set', { namespace: args.namespace, kind: 'Secret', dryRun: args.dryRun });
        const client = clients.coreV1;
        const existing = await client.readNamespacedSecret({ name: args.name, namespace: args.namespace }).catch(() => null);
        const data = {};
        for (const [k, v] of Object.entries(args.data)) {
            data[k] = args.base64Encoded ? v : Buffer.from(v, 'utf8').toString('base64');
        }
        const secret = {
            apiVersion: 'v1',
            kind: 'Secret',
            metadata: { name: args.name, namespace: args.namespace },
            type: args.type || 'Opaque',
            data,
        };
        const dryRun = args.dryRun !== false ? 'All' : undefined;
        if (!existing) {
            if (!args.createIfMissing) {
                return { content: [{ type: 'text', text: 'Secret does not exist and createIfMissing=false' }], isError: true };
            }
            const res = await client.createNamespacedSecret({ namespace: args.namespace, body: secret, dryRun });
            return { content: [{ type: 'text', text: JSON.stringify({ created: true, name: res.metadata?.name, keys: Object.keys(data) }) }] };
        }
        const res = await client.replaceNamespacedSecret({ name: args.name, namespace: args.namespace, body: secret, dryRun });
        return { content: [{ type: 'text', text: JSON.stringify({ updated: true, name: res.metadata?.name, keys: Object.keys(data) }) }] };
    });
}
