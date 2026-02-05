import { Logger } from 'winston';
import { Config } from '@backstage/config';
import {
    TokenManager,
    UrlReader,
    DiscoveryService,
} from '@backstage/backend-common';
import { PluginDatabaseManager } from '@backstage/backend-common';
import { PluginTaskScheduler } from '@backstage/backend-tasks';
import { PermissionAuthorizer } from '@backstage/plugin-permission-common';

export type PluginEnvironment = {
    logger: Logger;
    database: PluginDatabaseManager;
    config: Config;
    reader: UrlReader;
    discovery: DiscoveryService;
    tokenManager: TokenManager;
    scheduler: PluginTaskScheduler;
    permissions: PermissionAuthorizer;
};
