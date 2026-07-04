package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

var kv *UltraKV

type valueBody struct {
	Value string `json:"value"`
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func kvHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/kv/")

	if strings.HasPrefix(path, "prefix/") {
		prefix := strings.TrimPrefix(path, "prefix/")
		results := kv.PrefixScan(prefix)
		list := make([]map[string]string, 0, len(results))
		for k, v := range results {
			list = append(list, map[string]string{"key": k, "value": v})
		}
		json.NewEncoder(w).Encode(list)
		return
	}

	key := path
	if key == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		v, ok := kv.Get(key)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"value": v})

	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		var vb valueBody
		if err := json.Unmarshal(body, &vb); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		kv.Set(key, vb.Value)
		select {
		case kv.flushCh <- struct{}{}:
		default:
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	case http.MethodDelete:
		kv.Del(key)
		select {
		case kv.flushCh <- struct{}{}:
		default:
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	var err error
	kv, err = NewUltraKV("godb_server.db.btree", "godb_server.db.wal")
	if err != nil {
		log.Fatalf("Failed to start UltraKV: %v", err)
	}
	defer kv.Close()

	http.HandleFunc("/kv/", kvHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("GOD-B HTTP server running on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
