import React from 'react';
import { Table, TableColumn, StatusOk, StatusError, StatusWarning } from '@backstage/core-components';

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

const data: InspectionRow[] = [
    { name: 'High Error Rate', category: 'Reliability', status: 'pass', severity: 'critical', description: 'Error rate < 1%' },
    { name: 'P99 Latency', category: 'Performance', status: 'fail', severity: 'warning', description: 'P99 > 200ms' },
    { name: 'Memory Usage', category: 'Resource', status: 'pass', severity: 'info', description: 'Memory < 80%' },
];

export const InspectionsTableComponent = () => {
    return (
        <Table
            title="Health Inspections"
            options={{ search: true, paging: false }}
            columns={columns}
            data={data}
        />
    );
};
