import React from 'react';
import { Table, TableColumn, StatusOk, StatusError, StatusWarning, Progress, ResponseErrorPanel } from '@backstage/core-components';
import { useApi, configApiRef } from '@backstage/core-plugin-api';
import { useAsync } from 'react-use';
import { useEntity } from '@backstage/plugin-catalog-react';

type InspectionRow = {
    name: string;
    category: string;
    status: 'pass' | 'fail' | 'warn';
    severity: string;
    description: string;
};

const columns: TableColumn<InspectionRow>[] = [
    {
        title: 'Status', field: 'status', render: row => {
            if (row.status === 'pass') return <StatusOk />;
            if (row.status === 'fail') return <StatusError />;
            return <StatusWarning />;
        }
    },
    { title: 'Name', field: 'name' },
    { title: 'Category', field: 'category' },
    { title: 'Severity', field: 'severity' },
    { title: 'Description', field: 'description' },
];

export const InspectionsTableComponent = () => {
    const configApi = useApi(configApiRef);
    const { entity } = useEntity();
    const baseUrl = configApi.getOptionalString('rcaApp.baseUrl') || 'http://localhost:8080';
    const apiKey = configApi.getOptionalString('rcaApp.apiKey') || 'dev-key';

    const { value, loading, error } = useAsync(async () => {
        const response = await fetch(`${baseUrl}/api/v1/applications/${entity.metadata.name}/inspections`, {
            headers: {
                'X-API-Key': apiKey,
            },
        });
        if (!response.ok) {
            throw new Error(`Failed to fetch inspections: ${response.statusText}`);
        }
        const data = await response.json();
        return data.inspections || [];
    }, [entity.metadata.name]);

    if (loading) return <Progress />;
    if (error) return <ResponseErrorPanel error={error} />;

    return (
        <Table
            title="Health Inspections"
            options={{ search: true, paging: false }}
            columns={columns}
            data={value || []}
        />
    );
};
