package ws

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
)

type OriginPolicy struct {
	allowed map[string]struct{}
}

func NewOriginPolicy(allowedOrigins []string) OriginPolicy {
	allowed := make(map[string]struct{})
	for _, rawOrigin := range allowedOrigins {
		origin, _, ok := canonicalOrigin(rawOrigin)
		if !ok {
			continue
		}
		allowed[origin] = struct{}{}
	}
	return OriginPolicy{allowed: allowed}
}

func (p OriginPolicy) Allow(r *http.Request) bool {
	rawOrigin := strings.TrimSpace(r.Header.Get("Origin"))
	if rawOrigin == "" {
		return true
	}

	origin, originHost, ok := canonicalOrigin(rawOrigin)
	if !ok {
		return false
	}
	if sameHost(originHost, r.Host) {
		return true
	}

	_, ok = p.allowed[origin]
	return ok
}

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request, userID string) {
	ServeWSWithOriginPolicy(hub, w, r, userID, NewOriginPolicy(nil))
}

func ServeWSWithOriginPolicy(hub *Hub, w http.ResponseWriter, r *http.Request, userID string, originPolicy OriginPolicy) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     originPolicy.Allow,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		hub:    hub,
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, 256),
		rooms:  make(map[string]struct{}),
	}

	hub.register <- client

	go client.WritePump()
	go client.ReadPump()
}

func canonicalOrigin(rawOrigin string) (string, string, bool) {
	trimmed := strings.TrimRight(strings.TrimSpace(rawOrigin), "/")
	if trimmed == "" || trimmed == "*" {
		return "", "", false
	}

	originURL, err := url.Parse(trimmed)
	if err != nil {
		return "", "", false
	}
	if originURL.Scheme != "http" && originURL.Scheme != "https" {
		return "", "", false
	}
	if originURL.Host == "" || originURL.User != nil || originURL.RawQuery != "" || originURL.Fragment != "" {
		return "", "", false
	}
	if originURL.Path != "" && originURL.Path != "/" {
		return "", "", false
	}

	scheme := strings.ToLower(originURL.Scheme)
	host := canonicalHost(originURL.Host)
	return scheme + "://" + host, host, true
}

func sameHost(originHost, requestHost string) bool {
	return originHost != "" && originHost == canonicalHost(requestHost)
}

func canonicalHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}
