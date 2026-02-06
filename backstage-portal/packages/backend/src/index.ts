import Router from 'express-promise-router';
import {
    createServiceBuilder,
    loadBackendConfig,
    getRootLogger,
    useHotMemoize,
    notFoundHandler,
    CacheManager,
    DatabaseManager,
    SingleHostDiscovery,
    UrlReaders,
    ServerTokenManager,
} from '@backstage/backend-common';
import { Config } from '@backstage/config';
import auth from './plugins/auth';
import catalog from './plugins/catalog';
import proxy from './plugins/proxy';
import { PluginEnvironment } from './types';
import { ServerPermissionClient } from '@backstage/plugin-permission-node';
import { DefaultSchedulerService } from '@backstage/backend-tasks';

function makeCreateEnv(config: Config) {
    const root = getRootLogger();
    const reader = UrlReaders.default({ logger: root, config });
    const discovery = SingleHostDiscovery.fromConfig(config);
    const cacheManager = CacheManager.fromConfig(config);
    const databaseManager = DatabaseManager.fromConfig(config);
    const tokenManager = ServerTokenManager.fromConfig(config, { logger: root });
    const permissions = ServerPermissionClient.fromConfig(config, {
        discovery,
        tokenManager,
    });
    const scheduler = DefaultSchedulerService.fromConfig(config, { databaseManager });

    return (plugin: string): PluginEnvironment => {
        const logger = root.child({ type: 'plugin', plugin });
        const database = databaseManager.forPlugin(plugin);
        return {
            logger,
            database,
            config,
            reader,
            discovery,
            tokenManager,
            permissions,
            scheduler,
        };
    };
}

async function main() {
    const config = await loadBackendConfig({
        argv: process.argv,
        logger: getRootLogger(),
    });
    const createEnv = makeCreateEnv(config);

    const authEnv = createEnv('auth');
    const catalogEnv = createEnv('catalog');
    const proxyEnv = createEnv('proxy');

    const apiRouter = Router();
    apiRouter.use('/auth', await auth(authEnv));
    apiRouter.use('/catalog', await catalog(catalogEnv));
    apiRouter.use('/proxy', await proxy(proxyEnv));

    const service = createServiceBuilder(module)
        .loadConfig(config)
        .addRouter('/api', apiRouter)
        .addRouter('', notFoundHandler());

    await service.start().catch(err => {
        console.error(err);
        process.exit(1);
    });
}

module.hot?.accept();
main().catch(error => {
    console.error(`Backend failed to start up, ${error}`);
    process.exit(1);
});
