import React, { useState } from 'react';
import { Button, Card, CardContent, Typography, CircularProgress } from '@material-ui/core';
import { Alert } from '@material-ui/lab';

export const AIAnalysisComponent = () => {
    const [loading, setLoading] = useState(false);
    const [result, setResult] = useState<any>(null);

    const handleAnalyze = async () => {
        setLoading(true);
        // Simulate API call
        setTimeout(() => {
            setResult({
                root_cause: "Database Connection Pool Exhaustion",
                confidence: 0.92,
                reasoning: "Correlation found between 500 errors and high active connection count.",
                remediation: ["Increase max_connections in Postgres", "Check for leaked connections in Service A"]
            });
            setLoading(false);
        }, 1500);
    };

    return (
        <Card>
            <CardContent>
                <Typography variant="h5" gutterBottom>Root Cause Analysis</Typography>
                <Button
                    variant="contained"
                    color="primary"
                    onClick={handleAnalyze}
                    disabled={loading}
                >
                    {loading ? "Analyzing..." : "Analyze Now"}
                </Button>

                {loading && <CircularProgress style={{ marginLeft: 20 }} size={24} />}

                {result && (
                    <div style={{ marginTop: 20 }}>
                        <Alert severity={result.confidence > 0.8 ? "error" : "warning"}>
                            <Typography variant="h6">Root Cause Detected: {result.root_cause}</Typography>
                        </Alert>
                        <div style={{ marginTop: 10 }}>
                            <Typography variant="subtitle1"><strong>Confidence:</strong> {(result.confidence * 100).toFixed(0)}%</Typography>
                            <Typography variant="body1" paragraph>{result.reasoning}</Typography>
                            <Typography variant="subtitle2">Remediation Steps:</Typography>
                            <ul>
                                {result.remediation.map((step: string, i: number) => (
                                    <li key={i}>{step}</li>
                                ))}
                            </ul>
                        </div>
                    </div>
                )}
            </CardContent>
        </Card>
    );
};
