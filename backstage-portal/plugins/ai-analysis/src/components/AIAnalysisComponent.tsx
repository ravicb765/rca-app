import React, { useState } from 'react';
import { Button, Card, CardContent, Typography, CircularProgress, Divider, Box } from '@material-ui/core';
import { Alert } from '@material-ui/lab';
import { useApi, configApiRef } from '@backstage/core-plugin-api';
import { useEntity } from '@backstage/plugin-catalog-react';

const AI_ICON = "https://img.icons8.com/nolan/64/artificial-intelligence.png";

export const AIAnalysisComponent = () => {
    const configApi = useApi(configApiRef);
    const { entity } = useEntity();
    const [loading, setLoading] = useState(false);
    const [result, setResult] = useState<any>(null);
    const [error, setError] = useState<string | null>(null);

    const baseUrl = configApi.getOptionalString('rcaApp.baseUrl') || 'http://localhost:8080';
    const apiKey = configApi.getOptionalString('rcaApp.apiKey') || 'dev-key';

    const handleAnalyze = async () => {
        setLoading(true);
        setError(null);
        try {
            const response = await fetch(`${baseUrl}/api/v1/analyze`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-API-Key': apiKey,
                },
                body: JSON.stringify({
                    application_id: entity.metadata.name,
                    start_time: new Date(Date.now() - 3600000).toISOString(), // Last hour
                    end_time: new Date().toISOString(),
                }),
            });

            if (!response.ok) {
                throw new Error(`Analysis failed: ${response.statusText}`);
            }

            const data = await response.json();
            setResult(data);
        } catch (err: any) {
            setError(err.message);
        } finally {
            setLoading(false);
        }
    };

    return (
        <Card variant="outlined">
            <CardContent>
                <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
                    <Box display="flex" alignItems="center" gap={2}>
                        <img src={AI_ICON} width="32" height="32" alt="AI" />
                        <Typography variant="h5">AI Root Cause Analysis</Typography>
                    </Box>
                    <Button
                        variant="contained"
                        color="primary"
                        onClick={handleAnalyze}
                        disabled={loading}
                    >
                        {loading ? "Analyzing..." : "Run Analysis"}
                    </Button>
                </Box>

                <Divider />

                {loading && (
                    <Box display="flex" flexDirection="column" alignItems="center" my={4}>
                        <CircularProgress size={40} />
                        <Typography variant="body2" style={{ marginTop: 16 }}>
                            Gathering telemetry and processing patterns...
                        </Typography>
                    </Box>
                )}

                {error && (
                    <Box my={2}>
                        <Alert severity="error">{error}</Alert>
                    </Box>
                )}

                {result && !loading && (
                    <Box mt={3}>
                        <Alert severity={result.confidence > 0.8 ? "error" : "warning"} variant="outlined">
                            <Typography variant="h6">
                                Root Cause: {result.root_cause}
                            </Typography>
                        </Alert>

                        <Box mt={2}>
                            <Typography variant="subtitle1">
                                <strong>Confidence Level:</strong> {(result.confidence * 100).toFixed(0)}%
                            </Typography>

                            <Box mt={2}>
                                <Typography variant="subtitle2" gutterBottom>Reasoning:</Typography>
                                <Typography variant="body1" paragraph>
                                    {result.reasoning}
                                </Typography>
                            </Box>

                            {result.remediation && result.remediation.length > 0 && (
                                <Box mt={2}>
                                    <Typography variant="subtitle2" gutterBottom>Recommended Remediation Steps:</Typography>
                                    <ul>
                                        {result.remediation.map((step: string, i: number) => (
                                            <li key={i}>
                                                <Typography variant="body2">{step}</Typography>
                                            </li>
                                        ))}
                                    </ul>
                                </Box>
                            )}
                        </Box>

                        <Box mt={2} display="flex" justifyContent="flex-end">
                            <Typography variant="caption" color="textSecondary">
                                Analyzed at: {new Date(result.timestamp).toLocaleString()}
                            </Typography>
                        </Box>
                    </Box>
                )}

                {!result && !loading && !error && (
                    <Box my={4} textAlign="center">
                        <Typography color="textSecondary">
                            Click "Run Analysis" to trigger AI-powered root cause analysis for this service.
                        </Typography>
                    </Box>
                )}
            </CardContent>
        </Card>
    );
};
