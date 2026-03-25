//go:build linux || darwin

package wasm

// #include <sys/mman.h>
import "C"
import "unsafe"

// adviseRelease hints to the OS that the Wasm linear memory slice is no longer needed,
// so physical pages can be reclaimed immediately rather than waiting for memory pressure.
// On Linux, MADV_DONTNEED returns pages to the OS right away (zeroed on next access).
// On macOS, MADV_DONTNEED marks pages as purgeable; the OS reclaims them under pressure.
func adviseRelease(mem []byte) {
	if len(mem) == 0 {
		return
	}
	C.madvise(unsafe.Pointer(&mem[0]), C.size_t(len(mem)), C.MADV_DONTNEED)
}
