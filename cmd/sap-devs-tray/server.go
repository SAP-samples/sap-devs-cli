package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
)

//go:embed frontend
var frontendFS embed.FS

type Server struct {
	Token     string
	ConfigDir string
	CacheDir  string
	listener  net.Listener
	mux       *http.ServeMux
}

func NewServer(configDir, cacheDir string) (*Server, error) {
	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	s := &Server{
		Token:     token,
		ConfigDir: configDir,
		CacheDir:  cacheDir,
		mux:       http.NewServeMux(),
	}

	frontendContent, _ := fs.Sub(frontendFS, "frontend")
	s.mux.Handle("/", http.FileServer(http.FS(frontendContent)))
	s.mux.HandleFunc("/api/state", s.requireToken(s.handleState))
	s.mux.HandleFunc("/api/sync", s.requireToken(s.handleSync))
	s.mux.HandleFunc("/api/inject", s.requireToken(s.handleInject))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s.listener = listener
	return s, nil
}

func (s *Server) Port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

func (s *Server) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", s.Port())
}

func (s *Server) PanelURL() string {
	return fmt.Sprintf("%s/?token=%s", s.URL(), s.Token)
}

func (s *Server) Start() error {
	return http.Serve(s.listener, s.mux)
}

func (s *Server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if token != s.Token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	state := ReadState(s.ConfigDir, s.CacheDir)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	go func() {
		cmd := exec.Command(sapDevsBinary(), "sync")
		cmd.Stdout = nil
		cmd.Stderr = nil
		_ = cmd.Run()
	}()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (s *Server) handleInject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	go func() {
		cmd := exec.Command(sapDevsBinary(), "inject", "--no-sync")
		cmd.Stdout = nil
		cmd.Stderr = nil
		_ = cmd.Run()
	}()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func sapDevsBinary() string {
	name := "sap-devs"
	if runtime.GOOS == "windows" {
		name = "sap-devs.exe"
	}
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return name
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
