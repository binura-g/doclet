package collab

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

const (
	messageUpdate   = "yjs_update"
	messageSnapshot = "yjs_snapshot"
	messagePresence = "presence"
)

type Server struct {
	hub    *Hub
	broker *NatsBroker
}

func NewServer(hub *Hub, broker *NatsBroker) *Server {
	return &Server{hub: hub, broker: broker}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/ws", s.handleWebsocket)
	return mux
}

func (s *Server) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	documentID := r.URL.Query().Get("document_id")
	clientID := r.URL.Query().Get("client_id")
	if documentID == "" || clientID == "" {
		http.Error(w, "missing document_id or client_id", http.StatusBadRequest)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		conn:       conn,
		send:       make(chan []byte, 256),
		documentID: documentID,
		clientID:   clientID,
	}

	s.hub.Register(client)
	log.Printf("client %s joined %s", clientID, documentID)

	go client.WritePump()
	client.ReadPump(func(msg Message) {
		s.handleClientMessage(client, msg)
	})

	s.hub.Unregister(client)
	close(client.send)
	log.Printf("client %s left %s", clientID, documentID)
}

func (s *Server) handleClientMessage(client *Client, msg Message) {
	switch msg.Type {
	case messageUpdate:
		if payload := mustMarshal(msg); payload != nil {
			s.hub.Broadcast(msg.DocumentID, payload, msg.ClientID)
		}
		if s.broker != nil {
			s.broker.Publish(SubjectForDocument(msg.DocumentID, "updates"), msg)
		}
	case messagePresence:
		if payload := mustMarshal(msg); payload != nil {
			s.hub.Broadcast(msg.DocumentID, payload, msg.ClientID)
		}
		if s.broker != nil {
			s.broker.Publish(SubjectForDocument(msg.DocumentID, "presence"), msg)
		}
	case messageSnapshot:
		if s.broker != nil {
			s.broker.Publish(SubjectForDocument(msg.DocumentID, "snapshots"), msg)
		}
	default:
		log.Printf("unknown message type: %s", msg.Type)
	}
}

func (s *Server) SubscribeNATS() error {
	if s.broker == nil {
		return nil
	}

	if _, err := s.broker.Subscribe("doclet.documents.*.updates", func(msg Message) {
		if payload := mustMarshal(msg); payload != nil {
			s.hub.Broadcast(msg.DocumentID, payload, msg.ClientID)
		}
	}); err != nil {
		return err
	}

	if _, err := s.broker.Subscribe("doclet.documents.*.presence", func(msg Message) {
		if payload := mustMarshal(msg); payload != nil {
			s.hub.Broadcast(msg.DocumentID, payload, msg.ClientID)
		}
	}); err != nil {
		return err
	}

	return nil
}

func mustMarshal(msg Message) []byte {
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("json marshal error: %v", err)
		return nil
	}
	return payload
}
