package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
)

// held хранит выделенные через /eat блоки памяти, чтобы GC их не освободил.
var (
	mu   sync.Mutex
	held [][]byte
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "ok")
}

// eatHandler выделяет mb мегабайт и держит их до конца жизни процесса.
// Каждая страница записывается, чтобы память была реально закоммичена
// (иначе ядро выдаст её лениво и лимит cgroup не сработает).
func eatHandler(w http.ResponseWriter, r *http.Request) {
	mb, err := strconv.Atoi(r.URL.Query().Get("mb"))
	if err != nil || mb <= 0 {
		http.Error(w, "usage: /eat?mb=N (N > 0)", http.StatusBadRequest)
		return
	}
	block := make([]byte, mb*1024*1024)
	for i := 0; i < len(block); i += 4096 {
		block[i] = 1
	}
	mu.Lock()
	held = append(held, block)
	total := 0
	for _, b := range held {
		total += len(b)
	}
	mu.Unlock()
	fmt.Fprintf(w, "ate %d MB, holding %d MB total\n", mb, total/1024/1024)
}

// burnHandler запускает горутину с бесконечным циклом — грузит одно ядро CPU.
func burnHandler(w http.ResponseWriter, r *http.Request) {
	go func() {
		runtime.LockOSThread()
		for {
		}
	}()
	fmt.Fprintln(w, "burning one core")
}

func main() {
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/eat", eatHandler)
	http.HandleFunc("/burn", burnHandler)
	log.Printf("api listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
