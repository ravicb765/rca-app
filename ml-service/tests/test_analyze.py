import json
import os
import sys

# Ensure project root is on sys.path so `from main import app` works in CI
sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))

from main import app


def test_analyze_endpoint():
    client = app.test_client()
    payload = {
        "application_id": "test-app",
        "metrics": {
            "cpu": 85.0,
            "memory": 90.0,
            "error_rate": 0.05
        }
    }
    resp = client.post('/analyze', data=json.dumps(payload), content_type='application/json')
    assert resp.status_code == 200
    data = resp.get_json()
    assert data['application_id'] == 'test-app'
    assert 'analysis' in data
    assert 'root_cause' in data['analysis']
    assert 'reasoning' in data['analysis']
    assert 'remediation' in data['analysis']
