import pytest
import json
import sys
import os
import io

# Add parent directory to path to import main
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from main import app

@pytest.fixture
def client():
    app.config['TESTING'] = True
    with app.test_client() as client:
        yield client

def test_health(client):
    """Test the health check endpoint"""
    response = client.get('/health')
    assert response.status_code == 200
    assert response.json == {"status": "healthy"}

def test_analyze_success(client):
    """Test the analyze endpoint with valid data"""
    payload = {
        "application_id": "db-app",
        # High latency (1000ms) should trigger database_slowdown
        "metrics": {"cpu": 20, "memory": 40, "latency": 1000, "error_rate": 0.1}
    }
    response = client.post('/analyze', json=payload)
    assert response.status_code == 200
    data = response.json
    assert data['application_id'] == "db-app"
    assert "analysis" in data
    assert data['analysis']['root_cause'] == "database_slowdown"

def test_analyze_missing_payload(client):
    """Test the analyze endpoint with empty payload"""
    response = client.post('/analyze', json={})
    assert response.status_code == 400
    assert "error" in response.json

def test_retrain_success(client):
    """Test the retrain endpoint with a valid CSV file"""
    csv_content = b"""cpu,memory,latency,error_rate,label
20,40,10,0.0,normal
90,80,200,0.5,resource_exhaustion"""
    
    data = {
        'file': (io.BytesIO(csv_content), 'train.csv')
    }
    
    response = client.post('/retrain', data=data, content_type='multipart/form-data')
    assert response.status_code == 200
    assert response.json['status'] == "success"
    stats = response.json['stats']
    assert stats['normal_samples'] == 1
    assert stats['incident_samples'] == 1