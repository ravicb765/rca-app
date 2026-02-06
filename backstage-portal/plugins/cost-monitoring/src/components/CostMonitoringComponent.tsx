import React from 'react';
import { Card, CardContent, Typography, Box, Divider, useTheme } from '@material-ui/core';
import { Progress, ResponseErrorPanel, InfoCard } from '@backstage/core-components';
import { useApi, configApiRef } from '@backstage/core-plugin-api';
import { useAsync } from 'react-use';
import { useEntity } from '@backstage/plugin-catalog-react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts';

const PROVIDER_LOGOS: Record<string, string> = {
    aws: "https://img.icons8.com/color/48/amazon-web-services.png",
    gcp: "https://img.icons8.com/color/48/google-cloud.png",
    azure: "https://img.icons8.com/color/48/azure-1.png",
};

export const CostMonitoringComponent = () => {
    const configApi = useApi(configApiRef);
    const { entity } = useEntity();
    const theme = useTheme();
    const baseUrl = configApi.getOptionalString('rcaApp.baseUrl') || 'http://localhost:8080';
    const apiKey = configApi.getOptionalString('rcaApp.apiKey') || 'dev-key';

    const { value, loading, error } = useAsync(async () => {
        const response = await fetch(`${baseUrl}/api/v1/applications/${entity.metadata.name}/costs`, {
            headers: {
                'X-API-Key': apiKey,
            },
        });
        if (!response.ok) {
            throw new Error(`Failed to fetch cost data: ${response.statusText}`);
        }
        return await response.json();
    }, [entity.metadata.name]);

    if (loading) return <Progress />;
    if (error) return <ResponseErrorPanel error={error} />;

    // Prepare data for the chart
    const chartData = (value?.data || []).map((point: any) => ({
        period: point.period,
        cost: point.cost,
    }));

    return (
        <InfoCard title="Cloud Cost Monitoring">
            <Box mb={2}>
                <Typography variant="h4" color="primary" style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    {value?.provider && PROVIDER_LOGOS[value.provider.toLowerCase()] && (
                        <img src={PROVIDER_LOGOS[value.provider.toLowerCase()]} width="32" height="32" alt={value.provider} />
                    )}
                    {value?.currency || '$'}{value?.total_cost?.toFixed(2) || '0.00'}
                </Typography>
                <Typography variant="caption" color="textSecondary">
                    Total cost for {entity.metadata.name} in the selected period.
                </Typography>
            </Box>

            <Divider />

            <Box mt={3} style={{ width: '100%', height: 300 }}>
                <Typography variant="subtitle2" gutterBottom>Cost Trend (Daily)</Typography>
                <ResponsiveContainer>
                    <LineChart data={chartData}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="period" />
                        <YAxis />
                        <Tooltip
                            contentStyle={{ backgroundColor: theme.palette.background.paper }}
                            labelStyle={{ color: theme.palette.text.primary }}
                        />
                        <Legend />
                        <Line
                            type="monotone"
                            dataKey="cost"
                            stroke={theme.palette.primary.main}
                            activeDot={{ r: 8 }}
                            name="Cost"
                        />
                    </LineChart>
                </ResponsiveContainer>
            </Box>

            <Box mt={2}>
                <Typography variant="body2" color="textSecondary">
                    <strong>Trend:</strong> {value?.trend_percent > 0 ? '+' : ''}{value?.trend_percent?.toFixed(1)}% vs previous period
                </Typography>
            </Box>
        </InfoCard>
    );
};
