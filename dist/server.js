import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import pino from 'pino';
import { getClients } from './k8s.js';
import { registerClusterTools } from './tools/cluster.js';
import { registerWorkloadTools } from './tools/workloads.js';
import { registerResourceTools } from './tools/resources.js';
import { registerSecretTools } from './tools/secrets.js';
const logger = pino({ level: process.env.LOG_LEVEL || 'info' });
async function main() {
    // Initialize clients first; will throw if kubeconfig invalid
    getClients(logger);
    const server = new McpServer({ name: 'mcp-k8s-server', version: '0.1.0' });
    registerClusterTools(server, logger);
    registerWorkloadTools(server, logger);
    registerResourceTools(server, logger);
    registerSecretTools(server, logger);
    const transport = new StdioServerTransport();
    await server.connect(transport);
}
main().catch((err) => {
    // eslint-disable-next-line no-console
    console.error('Failed to start MCP K8s server:', err?.message || err);
    process.exit(1);
});
