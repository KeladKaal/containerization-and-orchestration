// Command batch stands in for "nightly analytics": it burns CPU and holds
// memory forever and does nothing useful. It is the load shop is allowed to
// sacrifice when the cluster runs short.
//
//   - BATCH_CPU_WORKERS   goroutines that spin on the CPU (default 1);
//   - BATCH_MEMORY_MIB    how much memory to hold in the end (default 64);
//   - BATCH_MEMORY_STEP_MIB how much to add per second on the way there
//     (default 16).
//
// Memory grows in steps instead of all at once on purpose: the kubelet checks
// memory.available every ~10 seconds, and a slow climb lets it see node
// pressure and evict before the kernel OOM killer gets there first.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"shop/internal/app"
)

const mib = 1 << 20

var heldBytes = app.Gauge("batch_memory_held_bytes", "Memory batch has allocated and touched.")

func main() {
	workers := app.EnvInt("BATCH_CPU_WORKERS", 1)
	target := app.EnvInt("BATCH_MEMORY_MIB", 64)
	step := app.EnvInt("BATCH_MEMORY_STEP_MIB", 16)
	log.Printf("batch: %d cpu workers, holding %d MiB (+%d MiB/s)", workers, target, step)

	for range workers {
		go burn()
	}
	go hold(target, step)
	log.Fatal(app.Serve(http.NewServeMux()))
}

func burn() {
	x := uint64(1)
	for {
		// Pointless arithmetic the compiler can't throw away.
		x = x*6364136223846793005 + 1442695040888963407
		if x == 0 {
			os.Exit(1)
		}
	}
}

func hold(targetMiB, stepMiB int) {
	var chunks [][]byte
	for len(chunks) < targetMiB {
		for i := 0; i < stepMiB && len(chunks) < targetMiB; i++ {
			chunk := make([]byte, mib)
			// Touch every page: an untouched allocation is only virtual memory
			// and never shows up in the cgroup's working set.
			for p := 0; p < len(chunk); p += 4096 {
				chunk[p] = 1
			}
			chunks = append(chunks, chunk)
		}
		heldBytes.Set(float64(len(chunks) * mib))
		time.Sleep(time.Second)
	}
	log.Printf("batch: holding %d MiB", len(chunks))
	select {} // keep chunks reachable forever
}
