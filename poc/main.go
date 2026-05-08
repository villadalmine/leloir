package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed static
var staticFiles embed.FS

// ── Config ────────────────────────────────────────────────────────────────────

var holmesURL = getenv("HOLMES_URL", "http://holmesgpt-holmes.holmesgpt:80")
var holmesModel = getenv("HOLMES_MODEL", "gemma4-31b")
var holmesFallback = getenv("HOLMES_FALLBACK", "nemotron-super")
var listenAddr = getenv("LISTEN_ADDR", ":3001")

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ── SSE broadcast ─────────────────────────────────────────────────────────────

type broker struct {
	mu      sync.RWMutex
	clients map[chan Event]struct{}
}

type Event struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Time string `json:"time"`
}

var hub = &broker{clients: make(map[chan Event]struct{})}

func (b *broker) subscribe() chan Event {
	ch := make(chan Event, 32)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broker) unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

func (b *broker) publish(e Event) {
	e.Time = time.Now().Format("15:04:05")
	b.mu.RLock()
	for ch := range b.clients {
		select {
		case ch <- e:
		default:
		}
	}
	b.mu.RUnlock()
}

// ── Holmes client ─────────────────────────────────────────────────────────────

type holmesRequest struct {
	Ask   string `json:"ask"`
	Model string `json:"model"`
}

type holmesResponse struct {
	Analysis string `json:"analysis"` // campo real de Holmes API
	Detail   string `json:"detail"`
}

func callHolmes(question, model string) (holmesResponse, error) {
	body, _ := json.Marshal(holmesRequest{Ask: question, Model: model})
	resp, err := http.Post(holmesURL+"/api/chat", "application/json", bytes.NewReader(body))
	if err != nil {
		return holmesResponse{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var hr holmesResponse
	if err := json.Unmarshal(raw, &hr); err != nil {
		return holmesResponse{}, fmt.Errorf("respuesta inválida: %s", raw)
	}
	if hr.Detail != "" {
		return hr, fmt.Errorf("%s", hr.Detail)
	}
	if hr.Analysis == "" {
		return hr, fmt.Errorf("respuesta vacía de Holmes (raw: %s)", raw)
	}
	return hr, nil
}

func isRateLimit(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "RateLimitError") || strings.Contains(s, "rate-limited")
}

func askHolmes(question string) {
	hub.publish(Event{Type: "info", Text: "Investigando: " + question})

	hr, err := callHolmes(question, holmesModel)
	if err != nil {
		if isRateLimit(err) && holmesFallback != "" {
			log.Printf("modelo %s rate-limited, reintentando con %s", holmesModel, holmesFallback)
			hub.publish(Event{Type: "info", Text: fmt.Sprintf("Modelo %s saturado, usando %s…", holmesModel, holmesFallback)})
			hr, err = callHolmes(question, holmesFallback)
		}
		if err != nil {
			hub.publish(Event{Type: "error", Text: "Holmes error: " + err.Error()})
			return
		}
	}
	hub.publish(Event{Type: "answer", Text: hr.Analysis})
}

// ── Handlers ──────────────────────────────────────────────────────────────────

type amAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type amPayload struct {
	Alerts []amAlert `json:"alerts"`
}

// POST /webhook — Alertmanager webhook
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload amPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	for _, a := range payload.Alerts {
		if a.Status != "firing" {
			continue
		}
		name := a.Labels["alertname"]
		ns := a.Labels["namespace"]
		summary := a.Annotations["summary"]
		question := fmt.Sprintf(
			"Alert '%s' is firing in namespace '%s'. Summary: %s. "+
				"Investigate the root cause and suggest a fix.",
			name, ns, summary,
		)
		go askHolmes(question)
	}
	w.WriteHeader(http.StatusOK)
}

type askRequest struct {
	Question string `json:"question"`
}

// POST /ask — pregunta manual desde la UI
func handleAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Question == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	go askHolmes(req.Question)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"queued"}`))
}

// GET /events — SSE stream
func handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := hub.subscribe()
	defer hub.unsubscribe(ch)

	hub.publish(Event{Type: "connected", Text: "Leloir PoC conectado — esperando investigaciones"})

	for {
		select {
		case e := <-ch:
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// GET / — UI estática embebida en el binario
func handleUI(w http.ResponseWriter, r *http.Request) {
	sub, _ := fs.Sub(staticFiles, "static")
	http.FileServer(http.FS(sub)).ServeHTTP(w, r)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	http.HandleFunc("/webhook", handleWebhook)
	http.HandleFunc("/ask", handleAsk)
	http.HandleFunc("/events", handleSSE)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/", handleUI)

	log.Printf("Leloir PoC escuchando en %s", listenAddr)
	log.Printf("Holmes URL: %s  modelo: %s  fallback: %s", holmesURL, holmesModel, holmesFallback)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
