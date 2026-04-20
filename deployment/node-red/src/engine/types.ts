import type { Node, NodeDef } from 'node-red';

export interface DeployConfig extends NodeDef {
    kubeconfigContent: string;
    privateKeyContent: string;
    certificateContent: string;
    domainAddress: string;
    instanceName: string;
    oidcIssuerUrl: string;
    clientId: string;
    depPostgresql: boolean;
    depPgUser: string;
    depPgPassword: string;
    depPgDatabase: string;
    depPgPersist: boolean;
    depHydra: boolean;
    depHydraDsn: string;
    depHydraSystemSecret: string;
    depHydraLoginUrl: string;
    depHydraConsentUrl: string;
    depNats: boolean;
    depNeo4j: boolean;
    depNeo4jPassword: string;
    depNeo4jPersist: boolean;
}

export interface DeployNode extends Node {
    ingressExternalIp?: string;
    dcsUrl?: string;
}
