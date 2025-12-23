package server

import (
	"context"
	"encoding/gob"
	"errors"
	"log"
	"net/http"
	"sync"

	"wg-exchange/cmd/wge-server/processor"
	"wg-exchange/cmd/wge-server/terminator"
	"wg-exchange/models"
)

var (
	initErr error
	once    sync.Once
)

func addPeer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var creds models.Credentials
	log.Println("[Request] addr:", r.RemoteAddr, ", user-agent:", r.UserAgent())
	if r.Header.Get("Content-Type") != "application/octet-stream" {
		log.Println("unsupported media type")
		http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
		return
	}
	if err := gob.NewDecoder(r.Body).Decode(&creds); err != nil {
		log.Println("decode failure:", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if c, err := processor.AddKey(creds); err != nil {
		log.Println("addKey failure:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	} else {
		// explicit, unnecessary
		w.Header().Add("Content-Type", "application/octet-stream")
		if err := gob.NewEncoder(w).Encode(c); err != nil {
			log.Println("error encoding")
			http.Error(w, "error encoding", http.StatusInternalServerError)
		}
		log.Println("successfully accepted request")
	}
}

func InitServer(s models.WGEServer) error {
	once.Do(func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", addPeer)

		server := &http.Server{
			Addr:    s.ListenAddress.String(),
			Handler: mux,
		}

		shutdown := func(ctx context.Context, _ context.CancelFunc) {
			<-ctx.Done()
			if err := server.Shutdown(ctx); err != nil {
				log.Println("server shutdown failure")
			}
			log.Println("server shut down")
		}
		terminator.HookInto(shutdown)

		go func() {
			if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				log.Println("server failure...")
			}
		}()
	})
	return initErr
}
