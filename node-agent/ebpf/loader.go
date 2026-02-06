package ebpf

import (
	"fmt"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

// NetworkTracerObjects contains the eBPF objects for the network tracer.
type NetworkTracerObjects struct {
	KprobeTcpConnect *ebpf.Program `ebpf:"kprobe_tcp_connect"`
	Events           *ebpf.Map     `ebpf:"events"`
}

// Close closes the eBPF objects.
func (o *NetworkTracerObjects) Close() error {
	if o.Events != nil {
		o.Events.Close()
	}
	if o.KprobeTcpConnect != nil {
		o.KprobeTcpConnect.Close()
	}
	return nil
}

// LoadNetworkTracer loads the eBPF objects from the compiled object file.
func LoadNetworkTracer(objPath string) (*NetworkTracerObjects, error) {
	spec, err := ebpf.LoadCollectionSpec(objPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load spec: %w", err)
	}

	var objs NetworkTracerObjects
	if err := spec.LoadAndAssign(&objs, nil); err != nil {
		return nil, fmt.Errorf("failed to load objects: %w", err)
	}
	return &objs, nil
}

// AttachNetworkTracer attaches the kprobe to tcp_connect.
func AttachNetworkTracer(objs *NetworkTracerObjects) (link.Link, error) {
	return link.Kprobe("tcp_connect", objs.KprobeTcpConnect, nil)
}
