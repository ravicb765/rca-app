package ebpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf NetworkTracer network_tracer.c
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf HttpTracer http_tracer.c
