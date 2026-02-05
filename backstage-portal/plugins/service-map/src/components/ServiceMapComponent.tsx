import React, { useEffect, useState } from 'react';
import CytoscapeComponent from 'react-cytoscapejs';
import { useAsync } from 'react-use';
import { Progress, ResponseErrorPanel } from '@backstage/core-components';
import { useApi, configApiRef } from '@backstage/core-plugin-api';

export const ServiceMapComponent = () => {
    const configApi = useApi(configApiRef);
    const baseUrl = configApi.getOptionalString('rcaApp.baseUrl') || 'http://localhost:8080';

    const { value, loading, error } = useAsync(async () => {
        const response = await fetch(`${baseUrl}/api/v1/servicemap`, {
            headers: {
                'X-API-Key': configApi.getOptionalString('rcaApp.apiKey') || 'dev-key',
            },
        });
        if (!response.ok) {
            throw new Error(`Failed to fetch service map: ${response.statusText}`);
        }
        const data = await response.json();

        // Transform RCA-App format to Cytoscape format if needed
        // Assuming the backend returns { applications: {...}, connections: [...] }
        const nodes = Object.entries(data.applications || {}).map(([id, app]: [string, any]) => ({
            data: { id, label: app.name || id, type: 'service' }
        }));

        const edges = (data.connections || []).map((conn: any) => ({
            data: {
                source: conn.source,
                target: conn.destination,
                label: `${conn.requestRate.toFixed(1)} req/s`,
                errorRate: conn.errorRate
            }
        }));

        return { nodes, edges };
    }, []);

    if (loading) return <Progress />;
    if (error) return <ResponseErrorPanel error={error} />;

    const elements = [...(value?.nodes || []), ...(value?.edges || [])];

    return (
        <div data-testid="service-map-container" style={{ height: '600px', border: '1px solid #ccc', borderRadius: '8px', overflow: 'hidden' }}>
            <CytoscapeComponent
                elements={elements}
                style={{ width: '100%', height: '100%' }}
                layout={{
                    name: 'cose',
                    animate: true,
                    nodeRepulsion: 4000,
                    idealEdgeLength: 100,
                }}
                stylesheet={[
                    {
                        selector: 'node',
                        style: {
                            'background-color': '#007FFF',
                            'label': 'data(label)',
                            'color': '#fff',
                            'text-valign': 'center',
                            'text-halign': 'center',
                            'width': 60,
                            'height': 60,
                            'font-size': '10px'
                        }
                    },
                    {
                        selector: 'edge',
                        style: {
                            'width': 2,
                            'line-color': '#00BFFF',
                            'target-arrow-color': '#00BFFF',
                            'target-arrow-shape': 'triangle',
                            'curve-style': 'bezier',
                            'label': 'data(label)',
                            'font-size': '8px',
                            'text-rotation': 'autorotate'
                        }
                    },
                    {
                        selector: 'edge[errorRate > 0.05]',
                        style: {
                            'line-color': '#FF4500',
                            'target-arrow-color': '#FF4500'
                        }
                    }
                ]}
            />
        </div>
    );
};
