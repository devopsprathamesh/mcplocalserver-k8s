import { getClients } from '../k8s.js';
import { ListNamespacesParams, SetContextParams } from '../schemas.js';
import { rateLimit } from '../authz.js';
export function registerClusterTools(server, logger) {
    const clients = getClients(logger);
    server.tool('cluster.health', 'Get basic cluster health and version', async () => {
        rateLimit('cluster.health');
        const start = Date.now();
        const info = await clients.versionApi.getCode();
        const durationMs = Date.now() - start;
        logger.info({ tool: 'cluster.health', durationMs }, 'ok');
        return {
            content: [{ type: 'text', text: JSON.stringify({ status: 'ok', clusterVersion: info.gitVersion, serverAddress: clients.kubeConfig.getCurrentCluster()?.server, timestamp: new Date().toISOString() }) }],
        };
    });
    server.tool('cluster.listContexts', 'List kubeconfig contexts and current selection', async () => {
        rateLimit('cluster.listContexts');
        const contexts = clients.kubeConfig.getContexts();
        const current = clients.kubeConfig.getCurrentContext();
        return {
            content: [{ type: 'text', text: JSON.stringify({ current, contexts: contexts.map((c) => ({ name: c.name, cluster: c.cluster, user: c.user })) }) }],
        };
    });
    server.tool('cluster.setContext', 'Set current kube context', SetContextParams.shape, async (args) => {
        rateLimit('cluster.setContext');
        const { context } = args;
        const ctx = clients.kubeConfig.getContexts().find((c) => c.name === context);
        if (!ctx) {
            return { content: [{ type: 'text', text: `Context ${context} not found` }], isError: true };
        }
        clients.setContext(context);
        return { content: [{ type: 'text', text: JSON.stringify({ current: context }) }] };
    });
    server.tool('ns.listNamespaces', 'List namespaces', ListNamespacesParams.shape, async (args) => {
        rateLimit('ns.listNamespaces');
        const { limit } = args;
        const res = await clients.coreV1.listNamespace({});
        const namespaces = res.items.map((ns) => ({
            name: ns.metadata?.name || '',
            status: ns.status?.phase || 'Unknown',
            age: ns.metadata?.creationTimestamp,
        }));
        return {
            content: [{ type: 'text', text: JSON.stringify({ namespaces: typeof limit === 'number' ? namespaces.slice(0, limit) : namespaces }) }],
        };
    });
}
