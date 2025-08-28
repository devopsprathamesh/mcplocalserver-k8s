import { getClients } from '../k8s.js';
import { ExecParams, GetPodParams, ListPodsParams, LogsParams } from '../schemas.js';
import { enforceMutatingGuards, rateLimit } from '../authz.js';
function summarizePod(pod) {
    const name = pod.metadata?.name || '';
    const namespace = pod.metadata?.namespace || '';
    const phase = pod.status?.phase || 'Unknown';
    const node = pod.spec?.nodeName || '';
    const restarts = pod.status?.containerStatuses?.reduce((a, c) => a + (c.restartCount || 0), 0) || 0;
    const age = pod.metadata?.creationTimestamp;
    return { name, namespace, phase, node, restarts, age };
}
export function registerWorkloadTools(server, logger) {
    const clients = getClients(logger);
    server.tool('pods.listPods', 'List pods with optional selectors', ListPodsParams.shape, async (args) => {
        rateLimit('pods.listPods');
        const ns = args.namespace || clients.defaultNamespace;
        const res = await clients.coreV1.listNamespacedPod({ namespace: ns, labelSelector: args.labelSelector, fieldSelector: args.fieldSelector, limit: args.limit });
        let rows = res.items.map(summarizePod);
        if (args.limit && rows.length > args.limit)
            rows = rows.slice(0, args.limit);
        return { content: [{ type: 'text', text: JSON.stringify({ pods: rows }) }] };
    });
    server.tool('pods.get', 'Get a pod summary including containers and events', GetPodParams.shape, async (args) => {
        rateLimit('pods.get');
        const pod = await clients.coreV1.readNamespacedPod({ name: args.name, namespace: args.namespace });
        const containers = pod.spec?.containers?.map((c) => ({ name: c.name, image: c.image })) || [];
        const initContainers = pod.spec?.initContainers?.map((c) => ({ name: c.name, image: c.image })) || [];
        // Events: best-effort using fieldSelector
        let events = [];
        try {
            const ev = await clients.coreV1.listNamespacedEvent({ namespace: args.namespace, fieldSelector: `involvedObject.name=${args.name}` });
            events = ev.items.slice(-10).map((e) => ({
                type: e.type,
                reason: e.reason,
                message: e.message,
                age: e.eventTime || e.lastTimestamp || e.firstTimestamp || undefined,
            }));
        }
        catch { }
        return {
            content: [{ type: 'text', text: JSON.stringify({ metadata: { name: pod.metadata?.name, namespace: pod.metadata?.namespace, uid: pod.metadata?.uid, creationTimestamp: pod.metadata?.creationTimestamp, labels: pod.metadata?.labels }, status: { phase: pod.status?.phase, podIP: pod.status?.podIP, hostIP: pod.status?.hostIP, conditions: pod.status?.conditions?.slice(-5) }, containers, initContainers, events }) }],
        };
    });
    server.tool('pods.logs', 'Get pod logs (tail by default)', LogsParams.shape, async (args) => {
        rateLimit('pods.logs');
        const ns = args.namespace;
        const name = args.name;
        const container = args.container || '';
        const { Writable } = await import('node:stream');
        const buffers = [];
        const writable = new Writable({
            write(chunk, _enc, cb) {
                buffers.push(Buffer.from(chunk));
                cb();
            },
        });
        await clients.log.log(ns, name, container, writable, {
            tailLines: args.tailLines ?? 200,
            sinceSeconds: args.sinceSeconds,
            timestamps: args.timestamps,
        });
        const text = Buffer.concat(buffers).toString('utf8');
        const lines = text ? text.split('\n') : [];
        const limited = lines.length > 1000 ? lines.slice(-1000) : lines;
        return { content: [{ type: 'text', text: limited.join('\n') }] };
    });
    server.tool('pods.exec', 'Execute a command in a pod', ExecParams.shape, async (args) => {
        rateLimit('pods.exec');
        enforceMutatingGuards(logger, 'pods.exec', { namespace: args.namespace, kind: 'Pod', dryRun: args.dryRun });
        const { namespace, name, container, command, timeoutSeconds } = args;
        const code = await new Promise((resolve, reject) => {
            clients.exec
                .exec(namespace, name, container || '', command, null, null, null, false, (status) => {
                const c = status?.status === 'Success' ? 0 : 1;
                resolve(c);
            })
                .catch((e) => reject(e));
        });
        return {
            content: [{ type: 'text', text: JSON.stringify({ exitCode: code }) }],
        };
    });
}
