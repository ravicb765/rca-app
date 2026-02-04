package main

import (
	"bufio"
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/cilium/ebpf"
)

// memMapping represents a memory mapping from /proc/<pid>/maps
type memMapping struct {
	start, end, offset uint64
	path               string
}

// elfCache holds cached symbols for a specific ELF file.
type elfCache struct {
	symbols []elf.Symbol
}

// UserSymbolCache manages symbol resolution for user-space processes.
type UserSymbolCache struct {
	mu          sync.Mutex
	pidCache    map[uint32][]memMapping // PID -> memory mappings
	elfCache    map[string]*elfCache    // File path -> ELF symbols
	ksyms       *KernelSymbols
	stackTraces *ebpf.Map
}

func NewUserSymbolCache(ksyms *KernelSymbols, stackMap *ebpf.Map) *UserSymbolCache {
	return &UserSymbolCache{
		pidCache:    make(map[uint32][]memMapping),
		elfCache:    make(map[string]*elfCache),
		ksyms:       ksyms,
		stackTraces: stackMap,
	}
}

// ResolveStack resolves a user or kernel stack ID into a folded stack string.
func (sc *UserSymbolCache) ResolveStack(stackID uint32, pid uint32, isKernel bool) string {
	if stackID <= 0 || sc.stackTraces == nil {
		return ""
	}

	var stack []uint64 = make([]uint64, 127)
	if err := sc.stackTraces.Lookup(stackID, &stack); err != nil {
		return ""
	}

	var resolved []string
	for _, ip := range stack {
		if ip == 0 {
			break
		}
		if isKernel {
			resolved = append(resolved, sc.ksyms.Resolve(ip))
		} else {
			resolved = append(resolved, sc.resolveUserAddr(pid, ip))
		}
	}

	// Reverse for FlameGraph format
	for i, j := 0, len(resolved)-1; i < j; i, j = i+1, j-1 {
		resolved[i], resolved[j] = resolved[j], resolved[i]
	}
	return strings.Join(resolved, ";")
}

func (sc *UserSymbolCache) resolveUserAddr(pid uint32, addr uint64) string {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	mappings, ok := sc.pidCache[pid]
	if !ok {
		var err error
		mappings, err = parseMaps(pid)
		if err != nil {
			return fmt.Sprintf("0x%x", addr)
		}
		sc.pidCache[pid] = mappings
	}

	for _, m := range mappings {
		if addr >= m.start && addr < m.end {
			// Address is in this mapping.
			cache, ok := sc.elfCache[m.path]
			if !ok {
				var err error
				cache, err = loadElfSymbols(m.path)
				if err != nil {
					// Cache failure to avoid retrying
					sc.elfCache[m.path] = nil
					return fmt.Sprintf("0x%x", addr)
				}
				sc.elfCache[m.path] = cache
			}
			if cache == nil {
				return fmt.Sprintf("0x%x", addr)
			}

			// Find the symbol
			relAddr := addr - m.start + m.offset
			for _, sym := range cache.symbols {
				if relAddr >= sym.Value && relAddr < sym.Value+sym.Size {
					return sym.Name
				}
			}
			return fmt.Sprintf("%s+0x%x", filepath.Base(m.path), relAddr)
		}
	}
	return fmt.Sprintf("0x%x", addr)
}

func parseMaps(pid uint32) ([]memMapping, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var mappings []memMapping
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 || !strings.HasPrefix(fields[5], "/") {
			continue
		}
		addrRange := strings.Split(fields[0], "-")
		start, _ := strconv.ParseUint(addrRange[0], 16, 64)
		end, _ := strconv.ParseUint(addrRange[1], 16, 64)
		offset, _ := strconv.ParseUint(fields[2], 16, 64)
		mappings = append(mappings, memMapping{start: start, end: end, offset: offset, path: fields[5]})
	}
	return mappings, nil
}

func loadElfSymbols(path string) (*elfCache, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	syms, err1 := f.Symbols()
	dynsyms, err2 := f.DynamicSymbols()
	if err1 != nil && err2 != nil {
		return nil, fmt.Errorf("no symbols found")
	}

	return &elfCache{symbols: append(syms, dynsyms...)}, nil
}
