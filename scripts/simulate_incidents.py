import json
import time
import urllib.request
import urllib.error
import sys
import os

# Default to localhost if not specified
BASE_URL = os.environ.get("ML_SERVICE_URL", "http://localhost:5000")

def send_metrics(app_id, metrics):
    url = f"{BASE_URL}/analyze"
    payload = {
        "application_id": app_id,
        "metrics": metrics
    }
    
    try:
        req = urllib.request.Request(
            url, 
            data=json.dumps(payload).encode('utf-8'),
            headers={'Content-Type': 'application/json'}
        )
        
        with urllib.request.urlopen(req) as response:
            data = json.loads(response.read().decode('utf-8'))
            return data
    except urllib.error.URLError as e:
        print(f"Error connecting to {url}: {e}")
        return None

def simulate():
    scenarios = [
        {
            "name": "Normal Operation",
            "metrics": {"cpu": 45, "memory": 55, "latency": 25, "error_rate": 0.0}
        },
        {
            "name": "Database Slowdown",
            "metrics": {"cpu": 30, "memory": 45, "latency": 1200, "error_rate": 0.05}
        },
        {
            "name": "Resource Exhaustion",
            "metrics": {"cpu": 98, "memory": 92, "latency": 350, "error_rate": 0.2}
        },
        {
            "name": "Network Issue",
            "metrics": {"cpu": 25, "memory": 40, "latency": 45, "error_rate": 8.5}
        }
    ]

    print(f"Starting simulation against {BASE_URL}...\n")

    for scenario in scenarios:
        print(f"--- Simulating: {scenario['name']} ---")
        print(f"Input Metrics: {scenario['metrics']}")
        
        result = send_metrics("sim-app-1", scenario['metrics'])
        
        if result:
            analysis = result.get('analysis', {})
            print(f"RCA Result: {analysis.get('root_cause', 'unknown')}")
            print(f"Confidence: {analysis.get('confidence', 0.0):.2f}")
            print(f"Remediation: {analysis.get('remediation', [])}")
        else:
            print("Failed to get analysis.")
        
        print("\n")
        time.sleep(1)

if __name__ == "__main__":
    simulate()