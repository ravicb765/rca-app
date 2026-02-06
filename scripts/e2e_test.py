import requests
import urllib.request
import urllib.error
import time
import sys
import json

# Configuration
SERVER_URL = "http://localhost:8080"
ML_URL = "http://localhost:5000"
APP_ID = "e2e-test-service"

def check_health():
    print("Checking system health...")
    try:
        # Check Server
        r = requests.get(f"{SERVER_URL}/api/v1/applications")
        if r.status_code != 200:
            print(f"Server unhealthy: {r.status_code}")
            return False
        with urllib.request.urlopen(f"{SERVER_URL}/api/v1/applications") as r:
            if r.getcode() != 200:
                print(f"Server unhealthy: {r.getcode()}")
                return False   
        # Check ML Service
        r = requests.get(f"{ML_URL}/health")
        if r.status_code != 200:
            print(f"ML Service unhealthy: {r.status_code}")
        with urllib.request.urlopen(f"{ML_URL}/health") as r:
            if r.getcode() != 200:
                print(f"ML Service unhealthy: {r.getcode()}")
                return False
        print("System is healthy.")
        return True
    except Exception as e:
        print(f"Health check failed: {e}")
        return False

def generate_traffic(incident_type="normal"):
    print(f"Generating {incident_type} traffic for {APP_ID}...")
    
    # Define metrics based on scenario
    metrics = {
        "cpu": 20.0,
        "memory": 40.0,
        "latency": 20.0,
        "error_rate": 0.0
    }
    
    if incident_type == "latency":
        metrics["latency"] = 1200.0 # High latency
    elif incident_type == "cpu":
        metrics["cpu"] = 95.0 # High CPU
    elif incident_type == "error":
        metrics["error_rate"] = 5.0 # High error rate
        
    # Send to ML service directly to simulate analysis trigger
    # In a real scenario, this would come from the server aggregating agent data
    payload = {
        "application_id": APP_ID,
        "metrics": metrics
    }
    
    try:
        req = urllib.request.Request(
            f"{ML_URL}/analyze",
            data=json.dumps(payload).encode('utf-8'),
            headers={'Content-Type': 'application/json'}
        )
        with urllib.request.urlopen(req) as r:
            if r.getcode() == 200:
                return json.loads(r.read().decode('utf-8'))
            else:
                print(f"Failed to send traffic: {r.getcode()}")
                return None
    except urllib.error.URLError as e:
        print(f"Traffic generation failed: {e}")
        return None

def run_test():
    if not check_health():
        sys.exit(1)
        
    # 1. Baseline
    print("\n--- Phase 1: Baseline ---")
    result = generate_traffic("normal")
    if result:
        rc = result.get("analysis", {}).get("root_cause")
        print(f"RCA Result: {rc}")
        if rc != "normal_operation":
            print("FAILURE: Expected normal_operation")
            sys.exit(1)
    
    time.sleep(2)
    
    # 2. Trigger Incident (Database Slowdown via Latency)
    print("\n--- Phase 2: Trigger Incident (High Latency) ---")
    result = generate_traffic("latency")
    if result:
        analysis = result.get("analysis", {})
        rc = analysis.get("root_cause")
        conf = analysis.get("confidence")
        print(f"RCA Result: {rc} (Confidence: {conf})")
        
        if rc == "database_slowdown":
            print("SUCCESS: Correctly identified database slowdown")
        else:
            print(f"FAILURE: Expected database_slowdown, got {rc}")
            sys.exit(1)
            
    # 3. Verify Server API (Inspections)
    # This requires the server to have received data. For this E2E script, 
    # we are primarily testing the ML loop. 
    # To test the full pipeline, we would need to inject data into the 
    # agent/server ingestion endpoint and wait for async processing.
    
    print("\nE2E Test Completed Successfully.")

if __name__ == "__main__":
    run_test()