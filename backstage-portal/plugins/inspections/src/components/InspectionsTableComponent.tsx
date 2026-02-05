import React from 'react';
import { Table, TableColumn, StatusOk, StatusError, StatusWarning, Progress, ResponseErrorPanel } from '@backstage/core-components';
import {
    Language as NetworkIcon,
    FlashOn as PerformanceIcon,
    Security as SecurityIcon,
    Storage as DatabaseIcon,
    CheckCircle as AvailabilityIcon,
    Memory as ResourceIcon
} from '@material-ui/icons';
import { Box } from '@material-ui/core';
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
    {
        title: 'Category',
        field: 'category',
        render: row => {
            const iconProps = { fontSize: 'small' as const, style: { marginRight: 8 } };
            let icon = <PerformanceIcon {...iconProps} />;

            switch (row.category.toLowerCase()) {
                case 'network': icon = <NetworkIcon {...iconProps} style={{ ...iconProps.style, color: '#4FACFE' }} />; break;
                case 'database': icon = <DatabaseIcon {...iconProps} style={{ ...iconProps.style, color: '#FFB800' }} />; break;
                case 'security': icon = <SecurityIcon {...iconProps} style={{ ...iconProps.style, color: '#FF3D71' }} />; break;
                case 'resource': icon = <ResourceIcon {...iconProps} style={{ ...iconProps.style, color: '#00F2FE' }} />; break;
                case 'availability': icon = <AvailabilityIcon {...iconProps} style={{ ...iconProps.style, color: '#32D74B' }} />; break;
            }

            return (
                <Box display="flex" alignItems="center">
                    {icon}
                    {row.category}
                </Box>
            );
        }
    },
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
