import * as k8s from '@kubernetes/client-node';
import YAML from 'yaml';
import { getClients } from '../k8s.js';
import { ApplyResourceParams, DeleteResourceParams, GetResourceParams } from '../schemas.js';
import { enforceMutatingGuards, rateLimit } from '../authz.js';
async function listOrGetGeneric(clients, args) {
    const koa = k8s.KubernetesObjectApi.makeApiClient(clients.kubeConfig);
    const apiVersion = args.group ? `${args.group}/${args.version}` : args.version;
    if (args.name) {
        const item = await koa.read({ apiVersion, kind: args.kind, metadata: { name: args.name, namespace: args.namespace || clients.defaultNamespace } });
        return { item };
    }
    const list = await koa.list(apiVersion, args.kind, args.namespace || clients.defaultNamespace, undefined, undefined, undefined, args.fieldSelector, args.labelSelector, args.limit);
    const summary = list.items.map((it) => ({ apiVersion: it.apiVersion, kind: it.kind, name: it.metadata?.name, namespace: it.metadata?.namespace, uid: it.metadata?.uid, creationTimestamp: it.metadata?.creationTimestamp }));
    return { items: summary };
}
export function registerResourceTools(server, logger) {
    const clients = getClients(logger);
    server.tool('resources.get', 'Get or list arbitrary resources by GVK', GetResourceParams.shape, async (args) => {
        rateLimit('resources.get');
        const result = await listOrGetGeneric(clients, args);
        return { content: [{ type: 'text', text: JSON.stringify(result) }] };
    });
    server.tool('resources.apply', 'Apply manifest YAML (server-side apply by default)', ApplyResourceParams.shape, async (args) => {
        rateLimit('resources.apply');
        const docs = YAML.parseAllDocuments(args.manifestYAML).map((d) => d.toJSON());
        const koa = k8s.KubernetesObjectApi.makeApiClient(clients.kubeConfig);
        const results = [];
        for (const obj of docs) {
            if (!obj || typeof obj !== 'object')
                continue;
            const { apiVersion, kind, metadata } = obj;
            if (!apiVersion || !kind || !metadata?.name) {
                results.push({ error: 'Invalid manifest: missing apiVersion/kind/metadata.name' });
                continue;
            }
            enforceMutatingGuards(logger, 'resources.apply', { namespace: metadata.namespace, kind });
            const applied = await k8s.KubernetesObjectApi.makeApiClient(clients.kubeConfig).patch(obj, undefined, args.dryRun !== false ? 'All' : undefined, args.fieldManager, true, k8s.PatchStrategy.ServerSideApply);
            results.push({ kind: applied.kind, name: applied.metadata?.name, namespace: applied.metadata?.namespace });
        }
        return { content: [{ type: 'text', text: JSON.stringify({ results }) }] };
    });
    server.tool('resources.delete', 'Delete a resource by GVK/name', DeleteResourceParams.shape, async (args) => {
        rateLimit('resources.delete');
        enforceMutatingGuards(logger, 'resources.delete', { namespace: args.namespace, kind: args.kind });
        const koa = k8s.KubernetesObjectApi.makeApiClient(clients.kubeConfig);
        const apiVersion = args.group ? `${args.group}/${args.version}` : args.version;
        const status = await koa.delete({ apiVersion, kind: args.kind, metadata: { name: args.name, namespace: args.namespace || clients.defaultNamespace } }, undefined, args.dryRun !== false ? 'All' : undefined, args.gracePeriodSeconds, undefined, args.propagationPolicy);
        return { content: [{ type: 'text', text: JSON.stringify({ status: status.status || 'Success' }) }] };
    });
}
