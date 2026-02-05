import React from 'react';
import { Route } from 'react-router-dom';
import { FlatRoutes } from '@backstage/core-app-api';
import {
    AppRouter,
    FeatureDiscoveryService,
    CatalogIndexPage,
    CatalogEntityPage,
} from '@backstage/plugin-catalog';
import { createApp } from '@backstage/app-defaults';
import { AppStatusPage } from '@backstage/core-components';
import { Root } from './components/Root';
import { EntityPage } from './components/catalog/EntityPage';

const app = createApp({
    apis: [],
    bindRoutes({ bind }) {
        bind(CatalogIndexPage.routes, {
            catalogEntity: CatalogEntityPage.routes.catalogEntity,
        });
    },
});

const App = () => (
    <AppRouter>
        <Root>
            <FlatRoutes>
                <Route path="/" element={<CatalogIndexPage />} />
                <Route path="/catalog" element={<CatalogIndexPage />} />
                <Route
                    path="/catalog/:namespace/:kind/:name"
                    element={<CatalogEntityPage />}
                >
                    <EntityPage />
                </Route>
            </FlatRoutes>
        </Root>
    </AppRouter>
);

export default app.createRoot(<App />);
