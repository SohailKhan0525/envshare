// envshare-server is a minimal server for envshare.
//
// It never sees plain, readable secrets. It only ever stores:
//   - each team member's name and public key
//   - a scrambled version of each member's access code (never the real one)
//   - locked secret files, organized by team and environment, each with an
//     optional expiry date
//   - a plain activity history, so an admin can see who did what and when
//
// All locking and unlocking happens on each person's own computer, using
// the envshare program (see cmd/envshare).
//
// To run it:
//
//	EnvshareAdminToken=changeme EnvshareDataDir=./data go run ./cmd/envshare-server
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func encodeBase64(b []byte) string          { return base64.StdEncoding.EncodeToString(b) }
func decodeBase64(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }

var validName = regexp.MustCompile(`^[a-zA-Z0-9]{1,64}$`)

type member struct {
	Name      string `json:"name"`
	PublicKey string `json:"publicKey"`
	TokenHash string `json:"tokenHash"`
}

type historyEntry struct {
	Time   string `json:"time"`
	Action string `json:"action"`
	Who    string `json:"who"`
	Detail string `json:"detail"`
}

type lockedSecret struct {
	Data      string `json:"data"`      // base64-encoded locked bytes
	ExpiresAt string `json:"expiresAt"` // RFC3339, empty means never
}

type teamStore struct {
	mu      sync.Mutex
	dataDir string
}

func newTeamStore(dataDir string) *teamStore {
	return &teamStore{dataDir: dataDir}
}

func (s *teamStore) teamDir(team string) string     { return filepath.Join(s.dataDir, "teams", team) }
func (s *teamStore) membersPath(team string) string  { return filepath.Join(s.teamDir(team), "members.json") }
func (s *teamStore) historyPath(team string) string  { return filepath.Join(s.teamDir(team), "history.json") }
func (s *teamStore) secretsDir(team string) string   { return filepath.Join(s.teamDir(team), "secrets") }

func (s *teamStore) loadMembers(team string) ([]member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadMembersLocked(team)
}

func (s *teamStore) loadMembersLocked(team string) ([]member, error) {
	data, err := os.ReadFile(s.membersPath(team))
	if os.IsNotExist(err) {
		return []member{}, nil
	}
	if err != nil {
		return nil, err
	}
	var members []member
	if err := json.Unmarshal(data, &members); err != nil {
		return nil, err
	}
	return members, nil
}

func (s *teamStore) saveMembersLocked(team string, members []member) error {
	if err := os.MkdirAll(s.teamDir(team), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(members, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.membersPath(team), data, 0o600)
}

func (s *teamStore) addMember(team, name, publicKey string) (token string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	members, err := s.loadMembersLocked(team)
	if err != nil {
		return "", err
	}
	for _, m := range members {
		if m.Name == name {
			return "", fmt.Errorf("a member named %q already exists", name)
		}
	}

	tokenBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, tokenBytes); err != nil {
		return "", err
	}
	token = hex.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))

	members = append(members, member{Name: name, PublicKey: publicKey, TokenHash: hex.EncodeToString(hash[:])})
	if err := s.saveMembersLocked(team, members); err != nil {
		return "", err
	}
	return token, nil
}

func (s *teamStore) removeMember(team, name string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	members, err := s.loadMembersLocked(team)
	if err != nil {
		return false, err
	}
	kept := members[:0]
	found := false
	for _, m := range members {
		if m.Name == name {
			found = true
			continue
		}
		kept = append(kept, m)
	}
	if !found {
		return false, nil
	}
	if err := s.saveMembersLocked(team, kept); err != nil {
		return false, err
	}
	return true, nil
}

func (s *teamStore) authenticate(team, token string) (string, bool) {
	members, err := s.loadMembers(team)
	if err != nil {
		return "", false
	}
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])
	for _, m := range members {
		if subtle.ConstantTimeCompare([]byte(m.TokenHash), []byte(hashHex)) == 1 {
			return m.Name, true
		}
	}
	return "", false
}

func (s *teamStore) addHistory(team, action, who, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.historyPath(team)
	var entries []historyEntry
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &entries)
	}
	entries = append(entries, historyEntry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		Action: action,
		Who:    who,
		Detail: detail,
	})
	if len(entries) > 1000 {
		entries = entries[len(entries)-1000:]
	}
	if err := os.MkdirAll(s.teamDir(team), 0o700); err == nil {
		data, _ := json.MarshalIndent(entries, "", "  ")
		_ = os.WriteFile(path, data, 0o600)
	}
}

func (s *teamStore) loadHistory(team string) ([]historyEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.historyPath(team))
	if os.IsNotExist(err) {
		return []historyEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []historyEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *teamStore) putSecret(team, env string, data []byte, expiresAt string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.secretsDir(team)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	wrapped := lockedSecret{Data: encodeBase64(data), ExpiresAt: expiresAt}
	out, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, env+".locked"), out, 0o600)
}

// getSecret returns the locked bytes, or ok=false with expired=true if the
// secret existed but is past its expiry date (and removes it).
func (s *teamStore) getSecret(team, env string) (data []byte, expired bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.secretsDir(team), env+".locked")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	var wrapped lockedSecret
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, false, err
	}
	if wrapped.ExpiresAt != "" {
		expiry, err := time.Parse(time.RFC3339, wrapped.ExpiresAt)
		if err == nil && time.Now().After(expiry) {
			_ = os.Remove(path)
			return nil, true, nil
		}
	}
	data, err = decodeBase64(wrapped.Data)
	return data, false, err
}

func (s *teamStore) listSecrets(team string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.secretsDir(team))
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".locked") {
			names = append(names, strings.TrimSuffix(e.Name(), ".locked"))
		}
	}
	return names, nil
}

type apiServer struct {
	store      *teamStore
	adminToken string
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeErr(w http.ResponseWriter, status int, msg string) { writeJSON(w, status, map[string]string{"error": msg}) }

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if strings.HasPrefix(h, prefix) {
		return strings.TrimPrefix(h, prefix)
	}
	return ""
}

func (a *apiServer) handleAddMember(w http.ResponseWriter, r *http.Request, team string) {
	if subtle.ConstantTimeCompare([]byte(bearerToken(r)), []byte(a.adminToken)) != 1 {
		writeErr(w, http.StatusUnauthorized, "that admin access code is not correct")
		return
	}
	var body struct {
		Name      string `json:"name"`
		PublicKey string `json:"publicKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "could not read the request")
		return
	}
	if !validName.MatchString(body.Name) {
		writeErr(w, http.StatusBadRequest, "please use only letters and numbers for the name")
		return
	}
	if !strings.HasPrefix(body.PublicKey, "age1") {
		writeErr(w, http.StatusBadRequest, "that does not look like a valid public key")
		return
	}
	token, err := a.store.addMember(team, body.Name, body.PublicKey)
	if err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	a.store.addHistory(team, "member added", "admin", body.Name)
	writeJSON(w, http.StatusCreated, map[string]string{"token": token})
}

func (a *apiServer) handleRemoveMember(w http.ResponseWriter, r *http.Request, team, name string) {
	if subtle.ConstantTimeCompare([]byte(bearerToken(r)), []byte(a.adminToken)) != 1 {
		writeErr(w, http.StatusUnauthorized, "that admin access code is not correct")
		return
	}
	found, err := a.store.removeMember(team, name)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "no member with that name")
		return
	}
	a.store.addHistory(team, "member removed", "admin", name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (a *apiServer) handleListMembers(w http.ResponseWriter, r *http.Request, team string) {
	if _, ok := a.store.authenticate(team, bearerToken(r)); !ok {
		writeErr(w, http.StatusUnauthorized, "that access code is not correct")
		return
	}
	members, err := a.store.loadMembers(team)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	type publicMember struct {
		Name      string `json:"name"`
		PublicKey string `json:"publicKey"`
	}
	out := make([]publicMember, 0, len(members))
	for _, m := range members {
		out = append(out, publicMember{Name: m.Name, PublicKey: m.PublicKey})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *apiServer) handlePutSecret(w http.ResponseWriter, r *http.Request, team, env string) {
	who, ok := a.store.authenticate(team, bearerToken(r))
	if !ok {
		writeErr(w, http.StatusUnauthorized, "that access code is not correct")
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 5<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "could not read the uploaded file")
		return
	}
	if len(data) == 0 {
		writeErr(w, http.StatusBadRequest, "the uploaded file was empty")
		return
	}
	expiresAt := ""
	if daysStr := r.URL.Query().Get("expireDays"); daysStr != "" {
		days, err := strconv.Atoi(daysStr)
		if err != nil || days <= 0 {
			writeErr(w, http.StatusBadRequest, "expire days must be a positive whole number")
			return
		}
		expiresAt = time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour).Format(time.RFC3339)
	}
	if err := a.store.putSecret(team, env, data, expiresAt); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail := env
	if expiresAt != "" {
		detail = env + " (expires " + expiresAt + ")"
	}
	a.store.addHistory(team, "secret pushed", who, detail)
	log.Printf("push: team=%s env=%s by=%s bytes=%d expires=%s", team, env, who, len(data), expiresAt)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *apiServer) handleGetSecret(w http.ResponseWriter, r *http.Request, team, env string) {
	who, ok := a.store.authenticate(team, bearerToken(r))
	if !ok {
		writeErr(w, http.StatusUnauthorized, "that access code is not correct")
		return
	}
	data, expired, err := a.store.getSecret(team, env)
	if os.IsNotExist(err) {
		writeErr(w, http.StatusNotFound, "there is no secret with that environment name yet")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if expired {
		a.store.addHistory(team, "secret expired", who, env)
		writeErr(w, http.StatusGone, "this secret has expired, ask an admin to push a fresh one")
		return
	}
	a.store.addHistory(team, "secret pulled", who, env)
	log.Printf("pull: team=%s env=%s by=%s bytes=%d", team, env, who, len(data))
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(data)
}

func (a *apiServer) handleListSecrets(w http.ResponseWriter, r *http.Request, team string) {
	if _, ok := a.store.authenticate(team, bearerToken(r)); !ok {
		writeErr(w, http.StatusUnauthorized, "that access code is not correct")
		return
	}
	names, err := a.store.listSecrets(team)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, names)
}

func (a *apiServer) handleHistory(w http.ResponseWriter, r *http.Request, team string) {
	if _, ok := a.store.authenticate(team, bearerToken(r)); !ok {
		writeErr(w, http.StatusUnauthorized, "that access code is not correct")
		return
	}
	entries, err := a.store.loadHistory(team)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (a *apiServer) router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/teams/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/teams/"), "/")
		if len(parts) < 2 || parts[0] == "" {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		team := parts[0]
		if !validName.MatchString(team) {
			writeErr(w, http.StatusBadRequest, "please use only letters and numbers for the team name")
			return
		}
		resource := parts[1]

		switch {
		case resource == "members" && len(parts) == 2 && r.Method == http.MethodPost:
			a.handleAddMember(w, r, team)
		case resource == "members" && len(parts) == 2 && r.Method == http.MethodGet:
			a.handleListMembers(w, r, team)
		case resource == "members" && len(parts) == 3 && r.Method == http.MethodDelete:
			a.handleRemoveMember(w, r, team, parts[2])
		case resource == "history" && len(parts) == 2 && r.Method == http.MethodGet:
			a.handleHistory(w, r, team)
		case resource == "secrets" && len(parts) == 2 && r.Method == http.MethodGet:
			a.handleListSecrets(w, r, team)
		case resource == "secrets" && len(parts) == 3 && r.Method == http.MethodPut:
			env := parts[2]
			if !validName.MatchString(env) {
				writeErr(w, http.StatusBadRequest, "please use only letters and numbers for the environment name")
				return
			}
			a.handlePutSecret(w, r, team, env)
		case resource == "secrets" && len(parts) == 3 && r.Method == http.MethodGet:
			env := parts[2]
			if !validName.MatchString(env) {
				writeErr(w, http.StatusBadRequest, "please use only letters and numbers for the environment name")
				return
			}
			a.handleGetSecret(w, r, team, env)
		default:
			writeErr(w, http.StatusNotFound, "not found")
		}
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return mux
}

func main() {
	dataDir := os.Getenv("EnvshareDataDir")
	if dataDir == "" {
		dataDir = "./data"
	}
	adminToken := os.Getenv("EnvshareAdminToken")
	if adminToken == "" {
		log.Fatal("please set EnvshareAdminToken before starting the server (this is the admin access code)")
	}
	addr := os.Getenv("EnvshareAddr")
	if addr == "" {
		addr = ":8443"
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatalf("could not create the data folder: %v", err)
	}

	srv := &apiServer{store: newTeamStore(dataDir), adminToken: adminToken}

	log.Printf("envshare server listening on %s (data folder: %s)", addr, dataDir)
	log.Println("note: put this behind a proper secure address (https) before real use, for example with Caddy or Railway.")
	log.Fatal(http.ListenAndServe(addr, srv.router()))
}

