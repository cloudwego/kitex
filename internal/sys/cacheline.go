package sys

import _ "unsafe"

// Prefetch prefetches data from memory addr to cache
//
// AMD64: Produce PREFETCHT0 instruction
//
// ARM64: Produce PRFM instruction with PLDL1KEEP option
//
//go:linkname Prefetch internal/sys.Prefetch
func Prefetch(addr uintptr) {}

// PrefetchStreamed prefetches data from memory addr, with a hint that this data is being streamed.
// That is, it is likely to be accessed very soon, but only once. If possible, this will avoid polluting the cache.
//
// AMD64: Produce PREFETCHNTA instruction
//
// ARM64: Produce PRFM instruction with PLDL1STRM option
//
//go:linkname PrefetchStreamed internal/sys.PrefetchStreamed
func PrefetchStreamed(addr uintptr) {}
