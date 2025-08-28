import * as k8s from '@kubernetes/client-node';
import pino from 'pino';
export class KubeClients {
    static instance = null;
    kubeConfig;
    coreV1;
    appsV1;
    customObjects;
    versionApi;
    log;
    exec;
    portForward;
    defaultNamespace;
    constructor(kubeConfig, logger) {
        this.kubeConfig = kubeConfig;
        this.coreV1 = kubeConfig.makeApiClient(k8s.CoreV1Api);
        this.appsV1 = kubeConfig.makeApiClient(k8s.AppsV1Api);
        this.customObjects = kubeConfig.makeApiClient(k8s.CustomObjectsApi);
        this.versionApi = kubeConfig.makeApiClient(k8s.VersionApi);
        this.log = new k8s.Log(kubeConfig);
        this.exec = new k8s.Exec(kubeConfig);
        this.portForward = new k8s.PortForward(kubeConfig);
        this.defaultNamespace = process.env.K8S_NAMESPACE || 'default';
        logger.info({ ns: this.defaultNamespace }, 'Kubernetes clients initialized');
    }
    static getInstance(logger) {
        if (this.instance)
            return this.instance;
        const kc = new k8s.KubeConfig();
        // Load kubeconfig order: env KUBECONFIG (support ':'); in-cluster; default
        const kubeconfigEnv = process.env.KUBECONFIG;
        if (kubeconfigEnv) {
            const paths = kubeconfigEnv.split(':').filter(Boolean);
            if (paths.length > 1) {
                // Merge multiple kubeconfigs into one
                const base = new k8s.KubeConfig();
                paths.forEach((p) => base.loadFromFile(p));
                kc.mergeConfig(base);
            }
            else {
                kc.loadFromFile(paths[0]);
            }
        }
        else {
            try {
                kc.loadFromCluster();
            }
            catch {
                kc.loadFromDefault();
            }
        }
        // Optionally override context
        const desiredContext = process.env.K8S_CONTEXT;
        if (desiredContext) {
            kc.setCurrentContext(desiredContext);
        }
        const clients = new KubeClients(kc, pino({ level: process.env.LOG_LEVEL || 'info' }));
        this.instance = clients;
        return clients;
    }
    getCurrentContext() {
        return this.kubeConfig.getCurrentContext();
    }
    setContext(context) {
        this.kubeConfig.setCurrentContext(context);
    }
}
export function getClients(logger) {
    return KubeClients.getInstance(logger);
}
