package mcpserver

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/config"
)

const browserCallbackAddress = "127.0.0.1:43819"

type browserAuth struct {
	mu        sync.Mutex
	address   string
	server    *http.Server
	listener  net.Listener
	nonce     string
	started   time.Time
	completed bool
	lastError string
}

func newBrowserAuth() *browserAuth {
	return &browserAuth{address: browserCallbackAddress}
}

func (a *browserAuth) Start() (string, error) {
	clientID := strings.TrimSpace(os.Getenv("ALCLI_OAUTH_CLIENT_ID"))
	if clientID == "" {
		clientID = config.BrowserClientID
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.server != nil {
		return "", errors.New("browser login is already in progress")
	}
	listener, err := net.Listen("tcp", a.address)
	if err != nil {
		return "", fmt.Errorf("listen for AniList login callback: %w", err)
	}
	nonceBytes := make([]byte, 24)
	if _, err := rand.Read(nonceBytes); err != nil {
		listener.Close()
		return "", fmt.Errorf("generate login nonce: %w", err)
	}
	a.nonce = hex.EncodeToString(nonceBytes)
	a.started = time.Now()
	a.completed = false
	a.lastError = ""
	a.listener = listener
	callbackURL := "http://" + listener.Addr().String() + "/callback"
	mux := http.NewServeMux()
	mux.HandleFunc("GET /callback", a.callbackPage)
	mux.HandleFunc("POST /complete", a.complete)
	a.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	server := a.server
	go func() {
		_ = server.Serve(listener)
	}()
	time.AfterFunc(5*time.Minute, func() { a.stop(server) })
	params := url.Values{"client_id": {clientID}, "response_type": {"token"}, "redirect_uri": {callbackURL}, "state": {a.nonce}}
	return api.AnilistAuthURL + "?" + params.Encode(), nil
}

func (a *browserAuth) Status() (pending, completed bool, lastError string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.server != nil, a.completed, a.lastError
}

func (a *browserAuth) stop(server *http.Server) {
	if server == nil {
		return
	}
	a.mu.Lock()
	if a.server != server {
		a.mu.Unlock()
		return
	}
	if !a.completed && a.lastError == "" {
		a.lastError = "browser login expired"
	}
	a.server = nil
	a.listener = nil
	a.mu.Unlock()
	_ = server.Close()
}

var callbackPage = template.Must(template.New("callback").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>ALcli login</title></head><body><p id="status">Completing ALcli login…</p><script nonce="{{.Nonce}}">(async()=>{const params=new URLSearchParams(location.hash.slice(1));const token=params.get('access_token');const state=params.get('state');history.replaceState(null,'',location.pathname);if(!token||state!=='{{.Nonce}}'){document.getElementById('status').textContent='AniList login could not be verified.';return}try{const response=await fetch('/complete',{method:'POST',headers:{'Content-Type':'application/json','X-ALcli-Nonce':'{{.Nonce}}'},body:JSON.stringify({token})});document.getElementById('status').textContent=response.ok?'ALcli is connected. You can close this tab.':'ALcli could not save the login.'}catch{document.getElementById('status').textContent='ALcli could not complete the login.'}})();</script></body></html>`))

func (a *browserAuth) callbackPage(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	nonce := a.nonce
	active := a.server != nil
	a.mu.Unlock()
	if !active {
		http.Error(w, "login expired", http.StatusGone)
		return
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-"+nonce+"'; connect-src 'self'; base-uri 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = callbackPage.Execute(w, struct{ Nonce string }{nonce})
}

func (a *browserAuth) complete(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	server := a.server
	nonce := a.nonce
	a.mu.Unlock()
	if server == nil || r.Header.Get("Origin") != "http://"+r.Host || r.Header.Get("X-ALcli-Nonce") != nonce {
		http.Error(w, "invalid login callback", http.StatusForbidden)
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		http.Error(w, "invalid login response", http.StatusBadRequest)
		return
	}
	if len(body.Token) < 10 || len(body.Token) > 8192 || strings.ContainsAny(body.Token, " \t\r\n") {
		http.Error(w, "invalid access token", http.StatusBadRequest)
		return
	}
	cfg, err := config.Load()
	if err == nil {
		cfg.AccessToken = body.Token
		err = cfg.Save()
	}
	a.mu.Lock()
	if err != nil {
		a.lastError = "could not save AniList login"
	} else {
		a.completed = true
	}
	a.mu.Unlock()
	if err != nil {
		http.Error(w, "could not save login", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
	go a.stop(server)
}
