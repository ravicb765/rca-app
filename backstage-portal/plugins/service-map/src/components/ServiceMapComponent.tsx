import React from 'react';
import CytoscapeComponent from 'react-cytoscapejs';
import { useAsync } from 'react-use';
import { Progress, ResponseErrorPanel } from '@backstage/core-components';

export const ServiceMapComponent = () => {
    // Mock fetch for now, or use an API client
    const { value, loading, error } = useAsync(async () => {
        // In real app: fetch(`${baseUrl}/api/v1/servicemap`)
        return {
            nodes: [
                { data: { id: 'a', label: 'Service A' } },
                { data: { id: 'b', label: 'Service B' } }
            ],
            edges: [
                { data: { source: 'a', target: 'b', label: 'HTTP' } }
            ]
        };
    }, []);

    if (loading) return <Progress />;
    if (error) return <ResponseErrorPanel error={error} />;

    return (
        <div data-testid="service-map-container" style={{ height: '600px', border: '1px solid #ccc' }}>
            <h1>Service Dependency Graph</h1>
            <CytoscapeComponent
                elements={CytoscapeComponent.normalizeElements(value || { nodes: [], edges: [] })}
                style={{ width: '100%', height: '100%' }}
                layout={{ name: 'grid' }}
            />
        </div>
    );
};
