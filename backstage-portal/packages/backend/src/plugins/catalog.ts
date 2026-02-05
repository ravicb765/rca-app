import { CatalogBuilder } from '@backstage/plugin-catalog-backend';
import { LdapOrgReaderProcessor } from '@backstage/plugin-catalog-backend-module-ldap';
import { Router } from 'express';
import { PluginEnvironment } from '../types';

export default async function createPlugin(
    env: PluginEnvironment,
): Promise<Router> {
    const builder = await CatalogBuilder.create(env);

    // Add LDAP processor for AD/LDAP ingestion
    builder.addProcessor(
        LdapOrgReaderProcessor.fromConfig(env.config, {
            logger: env.logger,
            tokenManager: env.tokenManager,
        }),
    );

    const { processingEngine, router } = await builder.build();
    await processingEngine.start();
    return router;
}
