import json
import time
import urllib.request
import urllib.error
import os

BASE_URL = os.environ.get("RCA_APP_URL", "http://localhost:8080")

def send_event(src_ip, dst_ip, memory_usage, cpu_usage, disk_usage, io_load, active_conns, packet_loss, http_5xx, io_wait, swap_usage, restart_count, cpu_throttling, goroutine_count):
    url = f"{BASE_URL}/api/v1/agent/event"
    payload = {
        "connections": [
            {
                "src_ip": src_ip,
                "dst_ip": dst_ip,
                "src_port": 12345,
                "dst_port": 80,
                "protocol": "http",
                "request_rate": 10.0,
                "error_rate": 0.0,
                "latency": 20.0,
                "memory_usage": memory_usage,
                "cpu_usage": cpu_usage,
                "disk_usage": disk_usage,
                "io_load": io_load,
                "active_connections": active_conns,
                "packet_loss": packet_loss,
                "http_5xx_rate": http_5xx,
                "io_wait": io_wait,
                "swap_usage": swap_usage,
                "restart_count": restart_count,
                "cpu_throttling": cpu_throttling,
                "goroutine_count": goroutine_count
            }
        ]
    }
    
    try:
        req = urllib.request.Request(
            url, 
            data=json.dumps(payload).encode('utf-8'),
            headers={'Content-Type': 'application/json'}
        )
        
        with urllib.request.urlopen(req) as response:
            print(f"Sent event: Mem={memory_usage}MB CPU={cpu_usage}% Loss={packet_loss}% 5xx={http_5xx} IOWait={io_wait}% Swap={swap_usage}% Restarts={restart_count} Throttling={cpu_throttling}% Goroutines={goroutine_count}, Status={response.getcode()}")
    except urllib.error.URLError as e:
        print(f"Error connecting to {url}: {e}")

def main():
    print(f"Populating metrics to {BASE_URL}...")
    
    # Simulate a memory leak
    memory = 100.0
    cpu = 10.0
    disk = 50.0
    io_load = 1.0
    active_conns = 10.0
    packet_loss = 0.0
    http_5xx = 0.0
    io_wait = 0.0
    swap_usage = 0.0
    restart_count = 0.0
    cpu_throttling = 0.0
    goroutine_count = 100.0
    
    src = "10.0.0.1"
    dst = "10.0.0.2"
    
    for i in range(15):
        send_event(src, dst, memory, cpu, disk, io_load, active_conns, packet_loss, http_5xx, io_wait, swap_usage, restart_count, cpu_throttling, goroutine_count)
        memory += 50.0 # Increase memory
        cpu = min(100.0, cpu + 5.0)
        disk = min(100.0, disk + 2.0)
        io_load += 1.0
        active_conns += 10.0 # Increase connections
        packet_loss = min(100.0, packet_loss + 0.5) # Increase packet loss
        http_5xx = min(1.0, http_5xx + 0.01) # Increase 5xx rate
        io_wait = min(100.0, io_wait + 2.0) # Increase IO wait
        swap_usage = min(100.0, swap_usage + 1.0) # Increase swap
        restart_count += 1.0 # Increase restarts
        cpu_throttling = min(100.0, cpu_throttling + 1.0) # Increase throttling
        goroutine_count += 1000.0 # Increase goroutines
        time.sleep(1)

if __name__ == "__main__":
    main()