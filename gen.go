package ebpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf NetworkTracer network_tracer.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf HttpTracer http_tracer.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf OomTracer oom_tracer.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf ProcessTracer process_tracer.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf FileTracer file_tracer.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf TcpRetransTracer tcp_retrans_tracer.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf DnsTracer dns_tracer.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf KfreeSkbTracer kfree_skb_tracer.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf ProcessExitTracer process_exit_tracer.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf UdpTracer udp_tracer.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf PageFaultTracer page_fault_tracer.c
