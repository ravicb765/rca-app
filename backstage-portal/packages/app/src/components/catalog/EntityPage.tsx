import React from 'react';
import {
    EntityLayout,
    EntityAboutCard,
    EntitySwitch,
    EntityOrphanWarning,
    EntityProcessingErrorsPanel,
} from '@backstage/plugin-catalog-react';
import { Grid } from '@material-ui/core';
import { ServiceMapComponent } from '@rca-app/plugin-service-map';
import { AIAnalysisComponent } from '@rca-app/plugin-ai-analysis';
import { InspectionsTableComponent } from '@rca-app/plugin-inspections';
import { ProfilingComponent } from '@rca-app/plugin-profiling';
import { CostMonitoringComponent } from '@rca-app/plugin-cost-monitoring';
import { EntityKubernetesContent } from '@backstage/plugin-kubernetes';

const overviewContent = (
    <Grid container spacing={3} alignItems="stretch">
        <EntityOrphanWarning />
        <EntityProcessingErrorsPanel />

        <Grid item md={6}>
            <EntityAboutCard variant="gridItem" />
        </Grid>

        <Grid item md={6}>
            <InspectionsTableComponent />
        </Grid>

        <Grid item md={12}>
            <ServiceMapComponent />
        </Grid>
    </Grid>
);

const serviceEntityPage = (
    <EntityLayout>
        <EntityLayout.Route path="/" title="Overview">
            {overviewContent}
        </EntityLayout.Route>

        <EntityLayout.Route path="/kubernetes" title="Kubernetes">
            <EntityKubernetesContent />
        </EntityLayout.Route>

        <EntityLayout.Route path="/ai-analysis" title="AI Analysis">
            <AIAnalysisComponent />
        </EntityLayout.Route>

        <EntityLayout.Route path="/profiling" title="Profiling">
            <ProfilingComponent />
        </EntityLayout.Route>

        <EntityLayout.Route path="/cost" title="Cost">
            <CostMonitoringComponent />
        </EntityLayout.Route>
    </EntityLayout>
);

export const EntityPage = () => (
    <EntitySwitch>
        <EntitySwitch.Case if={isKind('component')} children={serviceEntityPage} />
    </EntitySwitch>
);

// Helper to determine entity kind
function isKind(kind: string) {
    return (entity: any) => entity.kind.toLowerCase() === kind.toLowerCase();
}
