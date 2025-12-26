package server

import (
	"context"
	"encoding/gob"
	"errors"
	"log"
	"net/http"

	"wg-exchange/cmd/wge-server/processor"
	"wg-exchange/cmd/wge-server/terminator"
	"wg-exchange/models"
)

type Server struct {
	store  *processor.Store
	server *http.Server
}

func (s *Server) addPeer(w http.ResponseWriter, r *http.Request) {
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
	if c, err := s.store.AddKey(creds); err != nil {
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

func (s *Server) StartServer(ctx context.Context, _ context.CancelFunc) {
	go s.listen()
	<-ctx.Done()
	if err := s.server.Shutdown(ctx); err != nil {
		log.Println("server shutdown failure")
	}
	log.Println("server shut down")
}

func (s *Server) listen() {
	if err := s.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Println("server failure...")
	}
}

func NewServer(wgeServConf models.WGEServer, store *processor.Store) (serv *Server, err error) {
	mux := http.NewServeMux()
	serv = &Server{
		store: store,
		server: &http.Server{
			Addr:    wgeServConf.ListenAddress.String(),
			Handler: mux,
		},
	}
	mux.HandleFunc("/", serv.addPeer)

	terminator.HookInto(serv.StartServer)

	go func() {
	}()
	return serv, nil
}
