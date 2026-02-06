#!/usr/bin/env python3
import time
import json
import random
import requests
import sys

# Configuration
SERVER_URL = "http://localhost:8080/api/v1/agent/event"
METADATA_URL = "http://localhost:8080/api/v1/metadata"

# Topology Definition
# Service: (IP, Port, Protocol)
SERVICES = {
    "frontend": ("10.0.0.1", 80, "http"),
    "backend": ("10.0.0.2", 8080, "http"),
    "postgres": ("10.0.0.3", 5432, "postgres"),
    "redis": ("10.0.0.4", 6379, "redis"),
    "external-api": ("1.1.1.1", 443, "https")
}

# Traffic Patterns: (Source, Dest, Probability)
FLOWS = [
    ("frontend", "backend", 1.0),
    ("backend", "postgres", 0.8),
    ("backend", "redis", 0.9),
    ("backend", "external-api", 0.1)
]

def generate_traffic():
    connections = []
    
    for src_name, dst_name, prob in FLOWS:
        if random.random() > prob:
            continue
            
        src_ip, _, _ = SERVICES[src_name]
        dst_ip, dst_port, protocol = SERVICES[dst_name]
        
        # Randomize stats
        req_rate = random.uniform(10, 100)
        error_rate = 0.0
        if random.random() < 0.05: # 5% chance of errors
            error_rate = random.uniform(0.01, 0.1)
            
        latency = random.uniform(5, 50) # ms
        if dst_name == "postgres":
            latency = random.uniform(10, 100)
            
        event = {
            "src_ip": src_ip,
            "dst_ip": dst_ip,
            "src_port": random.randint(10000, 60000), # Ephemeral port
            "dst_port": dst_port,
            "protocol": protocol,
            "request_rate": req_rate,
            "error_rate": error_rate,
            "latency": latency,
            "active_connections": random.randint(1, 10),
            "memory_usage": random.uniform(100, 500), # MB
            "cpu_usage": random.uniform(10, 80), # %
            "disk_usage": random.uniform(20, 60), # %
            "io_load": random.uniform(0.1, 2.0)
        }
        connections.append(event)
        
    return connections

def register_metadata():
    pods = []
    for name, (ip, _, _) in SERVICES.items():
        if name == "external-api": continue
        pods.append({
            "name": name,
            "namespace": "simulation",
            "ip": ip,
            "node": "sim-node-1"
        })
    
    try:
        requests.post(METADATA_URL, json={"pods": pods}, timeout=2)
        print("Registered service metadata.")
    except Exception as e:
        print(f"Failed to register metadata: {e}")

def main():
    print(f"Starting traffic simulation to {SERVER_URL}...")
    print("Press Ctrl+C to stop.")
    register_metadata()
    try:
        while True:
            events = generate_traffic()
            if not events:
                continue
                
            payload = {"connections": events}
            
            try:
                resp = requests.post(SERVER_URL, json=payload, timeout=2)
                if resp.status_code == 200:
                    print(f"Sent {len(events)} events. Server response: {resp.json()}")
                else:
                    print(f"Failed to send events: {resp.status_code} {resp.text}")
            except requests.exceptions.RequestException as e:
                print(f"Connection error: {e}")
                
            time.sleep(2) # Send batch every 2 seconds
            
    except KeyboardInterrupt:
        print("\nSimulation stopped.")

if __name__ == "__main__":
    main()