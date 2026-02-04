# node-agent

The node-agent is an eBPF-based collector that loads compiled BPF objects and attaches probes to kernel functions or tracepoints.

Usage

- Compile eBPF programs (C) into `.o` objects using clang:

  ```bash
  clang -O2 -target bpf -c node-agent/ebpf/examples/fixed_stack.c -o node-agent/ebpf/fixed_stack.o
  ```

- Build the agent (without eBPF support for testing):

  ```bash
  cd node-agent
  go build -o node-agent
  sudo ./node-agent
  ```

- To enable full eBPF behavior (load & attach), compile with the `ebpf` build tag and ensure `github.com/cilium/ebpf` is available in your environment. The host must be Linux with required tools and privileges.

  ```bash
  cd node-agent
  go build -tags ebpf -o node-agent-ebpf
  sudo ./node-agent-ebpf
  ```

Notes

- The agent will attempt to attach programs based on their section (e.g., `kprobe/tcp_connect`, `kretprobe/tcp_connect`, `tracepoint/net/net_dev_xmit`).
- A background heartbeat is sent to `$RCA_APP_ENDPOINT/api/v1/agent/heartbeat` every 15s with a JSON object describing loaded programs and maps.
- Cleanup is attempted on shutdown; if attachments fail you will see warnings in logs.

Safety

- Loading and attaching eBPF programs requires root privileges and appropriate kernel support. Use an isolated test VM (kernel >= 5.4) or privileged CI runner when running these steps.
