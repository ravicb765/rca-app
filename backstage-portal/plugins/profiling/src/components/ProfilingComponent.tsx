import React from 'react';
import { Card, CardContent, Typography, Box, Divider } from '@material-ui/core';
import { Progress, ResponseErrorPanel, InfoCard } from '@backstage/core-components';
import { useApi, configApiRef } from '@backstage/core-plugin-api';
import { useAsync } from 'react-use';
import { useEntity } from '@backstage/plugin-catalog-react';

export const ProfilingComponent = () => {
    const configApi = useApi(configApiRef);
    const { entity } = useEntity();
    const baseUrl = configApi.getOptionalString('rcaApp.baseUrl') || 'http://localhost:8080';
    const apiKey = configApi.getOptionalString('rcaApp.apiKey') || 'dev-key';

    const { value, loading, error } = useAsync(async () => {
        const response = await fetch(`${baseUrl}/api/v1/applications/${entity.metadata.name}/profiles`, {
            headers: {
                'X-API-Key': apiKey,
            },
        });
        if (!response.ok) {
            throw new Error(`Failed to fetch profiles: ${response.statusText}`);
        }
        return await response.json();
    }, [entity.metadata.name]);

    if (loading) return <Progress />;
    if (error) return <ResponseErrorPanel error={error} />;

    return (
        <InfoCard title="Continuous Profiling">
            <Typography variant="body2" color="textSecondary" paragraph>
                Real-time CPU and Memory profiling data captured via eBPF node agents.
            </Typography>
            <Divider />
            <Box mt={2} style={{ height: '400px', backgroundColor: '#f5f5f5', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                {/* 
                    In a real production environment, we would integrate a flamegraph library 
                    like d3-flame-graph or a custom Canvas-based renderer for the eBPF profile data.
                */}
                <Box textAlign="center">
                    <Typography variant="h6">Flamegraph Visualization</Typography>
                    <Typography variant="body2">[ eBPF Profile Data for {entity.metadata.name} ]</Typography>
                    <Box mt={2} p={2} border="1px dashed #ccc">
                        {value?.profiles && value.profiles.length > 0 ? (
                            <pre style={{ textAlign: 'left', fontSize: '10px' }}>
                                {JSON.stringify(value.profiles[0], null, 2)}
                            </pre>
                        ) : (
                            <Typography color="textSecondary">No profile data available for the selected period.</Typography>
                        )}
                    </Box>
                </Box>
            </Box>
        </InfoCard>
    );
};
