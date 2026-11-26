// Command api is the public HTTP API of the demo. It maps a role to Accumulo authorisations,
// calls the Java gateway with them, and keeps an audit log of reads and deletes.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sort"
	"time"
)

type Server struct {
	store Store
	audit *Audit
}

// caller is who made the request, from the X-Demo-Role and X-Demo-Patient headers.
// Demo only: there is no login, so anyone can send any role.
type caller struct {
	role, patient string
	auths         []string
}

type handler func(w http.ResponseWriter, r *http.Request, c caller)

// Patient is one row as the caller sees it. Cells maps "family:qualifier" to the value.
type Patient struct {
	ID    string            `json:"id"`
	Cells map[string]string `json:"cells"`
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /api/me", s.withCaller(me))
	mux.HandleFunc("GET /api/patients", s.withCaller(s.list))
	mux.HandleFunc("GET /api/patients/{id}", s.withCaller(s.get))
	mux.HandleFunc("DELETE /api/patients/{id}", s.withCaller(adminOnly(s.delete)))
	mux.HandleFunc("GET /api/patients/{id}/raw", s.withCaller(adminOnly(s.raw)))
	mux.HandleFunc("POST /api/compact", s.withCaller(adminOnly(s.compact)))
	mux.HandleFunc("GET /api/audit", s.withCaller(adminOnly(s.auditLog)))
	return mux
}

func (s *Server) withCaller(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := caller{role: r.Header.Get("X-Demo-Role"), patient: r.Header.Get("X-Demo-Patient")}
		if c.role == "" {
			writeError(w, http.StatusBadRequest, "set the X-Demo-Role header")
			return
		}
		auths, err := authsFor(c.role, c.patient)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		c.auths = auths
		if c.role != "Patient" {
			c.patient = ""
		}
		h(w, r, c)
	}
}

func adminOnly(h handler) handler {
	return func(w http.ResponseWriter, r *http.Request, c caller) {
		if c.role != "Admin" {
			writeError(w, http.StatusForbidden, "only Admin can do this")
			return
		}
		h(w, r, c)
	}
}

// me returns the caller's role and labels, so the web page can show them. It is not audited.
func me(w http.ResponseWriter, r *http.Request, c caller) {
	writeJSON(w, http.StatusOK, map[string]any{"role": c.role, "patient": c.patient, "auths": c.auths})
}

func (s *Server) list(w http.ResponseWriter, r *http.Request, c caller) {
	cells, err := s.store.Scan("", c.auths)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.log(c, "list", "")
	writeJSON(w, http.StatusOK, group(cells))
}

func (s *Server) get(w http.ResponseWriter, r *http.Request, c caller) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	cells, err := s.store.Scan(id, c.auths)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.log(c, "read", id)
	if len(cells) == 0 {
		writeError(w, http.StatusNotFound, id+" has no cells that you can read")
		return
	}
	writeJSON(w, http.StatusOK, group(cells)[0])
}

// delete is the first step of the right to erasure: it writes delete markers.
// The data stays in the files until a compaction. See raw and compact.
func (s *Server) delete(w http.ResponseWriter, r *http.Request, c caller) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	// The hold check reads with LEGAL itself, so the rule does not depend on the caller's labels.
	cells, err := s.store.Scan(id, []string{"LEGAL"})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(cells) == 0 {
		writeError(w, http.StatusNotFound, id+" has no data to delete")
		return
	}
	for _, cell := range cells {
		if cell.Family == "legal" && cell.Qualifier == "hold" && cell.Value == "true" {
			s.log(c, "delete-refused", id)
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": id + " is on legal hold, so it cannot be erased. " +
					"A legal hold means the record must be kept until the hold is lifted.",
				"legalHold": true,
			})
			return
		}
	}
	n, err := s.store.Delete(id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.log(c, "delete", id)
	writeJSON(w, http.StatusOK, map[string]int{"deleted": n})
}

// raw shows what is still stored in the files for a row, with every label and delete marker.
func (s *Server) raw(w http.ResponseWriter, r *http.Request, c caller) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	cells, err := s.store.Raw(id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.log(c, "raw", id)
	if cells == nil {
		cells = []Cell{}
	}
	writeJSON(w, http.StatusOK, cells)
}

func (s *Server) compact(w http.ResponseWriter, r *http.Request, c caller) {
	if err := s.store.Compact(); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.log(c, "compact", "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) auditLog(w http.ResponseWriter, r *http.Request, c caller) {
	writeJSON(w, http.StatusOK, s.audit.Recent())
}

func (s *Server) log(c caller, action, row string) {
	s.audit.Add(Entry{Time: time.Now().UTC(), Role: c.role, Patient: c.patient, Action: action, Row: row})
}

func pathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if err := checkPatientID(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return "", false
	}
	return id, true
}

// group turns cells into one Patient per row, sorted by id.
func group(cells []Cell) []Patient {
	byID := map[string]*Patient{}
	for _, c := range cells {
		p, ok := byID[c.Row]
		if !ok {
			p = &Patient{ID: c.Row, Cells: map[string]string{}}
			byID[c.Row] = p
		}
		p.Cells[c.Family+":"+c.Qualifier] = c.Value
	}
	out := make([]Patient, 0, len(byID))
	for _, p := range byID {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func main() {
	gw := os.Getenv("GATEWAY_URL")
	if gw == "" {
		gw = "http://gateway:8081"
	}
	s := &Server{store: NewGateway(gw), audit: &Audit{}}
	srv := &http.Server{Addr: ":8080", Handler: s.routes(), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("api listening on :8080, gateway %s", gw)
	log.Fatal(srv.ListenAndServe())
}
