//go:build !linux && !darwin

package wasm

// adviseRelease is a no-op on platforms without madvise(2).
func adviseRelease(_ []byte) {}
