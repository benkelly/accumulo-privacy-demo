package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// fakeStore filters cells by label the way Accumulo does, for the demo's labels only:
// "" (public), one label, or labels joined by all "&" or all "|". No brackets.
type fakeStore struct {
	cells   []Cell
	deleted []string
	old     []OldFile
}

func visible(label string, auths []string) bool {
	switch {
	case label == "":
		return true
	case strings.Contains(label, "&"):
		for _, l := range strings.Split(label, "&") {
			if !slices.Contains(auths, l) {
				return false
			}
		}
		return true
	default:
		for _, l := range strings.Split(label, "|") {
			if slices.Contains(auths, l) {
				return true
			}
		}
		return false
	}
}

func (f *fakeStore) Scan(row string, auths []string) ([]Cell, error) {
	var out []Cell
	for _, c := range f.cells {
		if (row == "" || c.Row == row) && visible(c.Visibility, auths) && !slices.Contains(f.deleted, c.Row) {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeStore) Delete(row string) (int, error) {
	f.deleted = append(f.deleted, row)
	return 1, nil
}

func (f *fakeStore) Compact() (int, error) { return len(f.old), nil }

func (f *fakeStore) OldFiles() ([]OldFile, error)   { return f.old, nil }
func (f *fakeStore) Raw(row string) ([]Cell, error) { return nil, nil }

// patient builds one row with the same families and labels as gateway/Seed.java.
func patient(id string, hold bool) []Cell {
	self := "PATIENT_p" + id[2:]
	h := "false"
	if hold {
		h = "true"
	}
	return []Cell{
		{Row: id, Family: "info", Qualifier: "name", Visibility: "PII", Value: "Name " + id},
		{Row: id, Family: "info", Qualifier: "county", Value: "Dublin"},
		{Row: id, Family: "info", Qualifier: "visits", Value: "4"},
		{Row: id, Family: "clinical", Qualifier: "diagnosis", Visibility: "CLINICIAN|" + self, Value: "asthma"},
		{Row: id, Family: "clinical", Qualifier: "notes", Visibility: "CLINICIAN", Value: "note"},
		{Row: id, Family: "legal", Qualifier: "case_note", Visibility: "LEGAL", Value: "case"},
		{Row: id, Family: "legal", Qualifier: "medical_evidence", Visibility: "LEGAL&CLINICIAN", Value: "letter"},
		{Row: id, Family: "legal", Qualifier: "hold", Visibility: "LEGAL", Value: h},
	}
}

func newTestServer() (*Server, *fakeStore) {
	store := &fakeStore{}
	store.cells = append(store.cells, patient("p-0001", false)...)
	store.cells = append(store.cells, patient("p-0002", false)...)
	store.cells = append(store.cells, patient("p-0009", true)...)
	return &Server{store: store, audit: &Audit{}}, store
}

func do(s *Server, method, path, role, patientID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if role != "" {
		req.Header.Set("X-Demo-Role", role)
	}
	if patientID != "" {
		req.Header.Set("X-Demo-Patient", patientID)
	}
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)
	return rec
}

func list(t *testing.T, s *Server, role, patientID string) map[string]map[string]string {
	t.Helper()
	rec := do(s, "GET", "/api/patients", role, patientID)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s list: status %d, body %s", role, rec.Code, rec.Body)
	}
	var ps []Patient
	if err := json.Unmarshal(rec.Body.Bytes(), &ps); err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]string{}
	for _, p := range ps {
		out[p.ID] = p.Cells
	}
	return out
}

func columns(cells map[string]string) []string {
	var cols []string
	for k := range cells {
		cols = append(cols, k)
	}
	slices.Sort(cols)
	return cols
}

func TestWhatEachRoleSees(t *testing.T) {
	tests := []struct {
		role, patient, row string
		want               []string
	}{
		{role: "Analyst", row: "p-0001", want: []string{"info:county", "info:visits"}},
		{role: "Clinician", row: "p-0001", want: []string{"clinical:diagnosis", "clinical:notes",
			"info:county", "info:name", "info:visits"}},
		{role: "Legal", row: "p-0001", want: []string{"info:county", "info:name", "info:visits",
			"legal:case_note", "legal:hold"}},
		{role: "Patient", patient: "p-0001", row: "p-0001", want: []string{"clinical:diagnosis",
			"info:county", "info:visits"}},
		{role: "Patient", patient: "p-0001", row: "p-0002", want: []string{"info:county", "info:visits"}},
		{role: "Admin", row: "p-0001", want: []string{"clinical:diagnosis", "clinical:notes",
			"info:county", "info:name", "info:visits", "legal:case_note", "legal:hold",
			"legal:medical_evidence"}},
	}
	for _, tt := range tests {
		s, _ := newTestServer()
		got := columns(list(t, s, tt.role, tt.patient)[tt.row])
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s %s sees %s on %s, want %s", tt.role, tt.patient, got, tt.row, tt.want)
		}
	}
}

func TestAnalystNeverGetsNames(t *testing.T) {
	s, _ := newTestServer()
	for id, cells := range list(t, s, "Analyst", "") {
		if _, ok := cells["info:name"]; ok {
			t.Errorf("Analyst got info:name for %s", id)
		}
	}
	rec := do(s, "GET", "/api/patients/p-0001", "Analyst", "")
	if strings.Contains(rec.Body.String(), "info:name") {
		t.Errorf("Analyst got info:name from GET /api/patients/p-0001: %s", rec.Body)
	}
}

func TestPatientSeesOnlyOwnDiagnosis(t *testing.T) {
	s, _ := newTestServer()
	for id, cells := range list(t, s, "Patient", "p-0001") {
		_, ok := cells["clinical:diagnosis"]
		if ok != (id == "p-0001") {
			t.Errorf("Patient p-0001 sees a diagnosis for %s: %v", id, ok)
		}
	}
}

func TestDeleteRefusedOnLegalHold(t *testing.T) {
	s, store := newTestServer()
	rec := do(s, "DELETE", "/api/patients/p-0009", "Admin", "")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "legal hold") {
		t.Fatalf("status %d, body %s, want 409 with a legal hold message", rec.Code, rec.Body)
	}
	if len(store.deleted) != 0 {
		t.Errorf("store deleted %v", store.deleted)
	}
	if got := s.audit.Recent()[0].Action; got != "delete-refused" {
		t.Errorf("audit action %q, want delete-refused", got)
	}
}

func TestDeleteWithoutHold(t *testing.T) {
	s, store := newTestServer()
	if rec := do(s, "DELETE", "/api/patients/p-0002", "Admin", ""); rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if !slices.Equal(store.deleted, []string{"p-0002"}) {
		t.Errorf("store deleted %v, want [p-0002]", store.deleted)
	}
	if rec := do(s, "GET", "/api/patients/p-0002", "Admin", ""); rec.Code != http.StatusNotFound {
		t.Errorf("read after delete: status %d, want 404", rec.Code)
	}
}

func TestAdminOnlyEndpoints(t *testing.T) {
	endpoints := [][2]string{
		{"DELETE", "/api/patients/p-0002"},
		{"GET", "/api/patients/p-0002/raw"},
		{"POST", "/api/compact"},
		{"GET", "/api/old-files"},
		{"GET", "/api/audit"},
	}
	for _, role := range []string{"Clinician", "Legal", "Analyst"} {
		for _, e := range endpoints {
			s, store := newTestServer()
			if rec := do(s, e[0], e[1], role, ""); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s as %s: status %d, want 403", e[0], e[1], role, rec.Code)
			}
			if len(store.deleted) != 0 {
				t.Errorf("%s deleted %v", role, store.deleted)
			}
		}
	}
}

func TestBadRequests(t *testing.T) {
	s, _ := newTestServer()
	tests := []struct{ path, role, patient string }{
		{"/api/patients", "", ""},
		{"/api/patients", "Root", ""},
		{"/api/patients", "Patient", ""},
		{"/api/patients", "Patient", "p-0099"},
		{"/api/patients/kyle", "Admin", ""},
	}
	for _, tt := range tests {
		if rec := do(s, "GET", tt.path, tt.role, tt.patient); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s role=%q patient=%q: status %d, want 400", tt.path, tt.role, tt.patient, rec.Code)
		}
	}
}

func TestAuditRecordsReads(t *testing.T) {
	s, _ := newTestServer()
	do(s, "GET", "/api/patients/p-0001", "Patient", "p-0001")
	do(s, "GET", "/api/patients", "Analyst", "")
	got := s.audit.Recent()
	if len(got) != 2 || got[0].Action != "list" || got[1].Row != "p-0001" || got[1].Patient != "p-0001" {
		t.Errorf("audit = %+v", got)
	}
}

func TestOldFilesCountsWhatIsStillOnDisk(t *testing.T) {
	s, store := newTestServer()
	store.old = []OldFile{{Path: "a.rf", Exists: true}, {Path: "b.rf", Exists: false}}
	if rec := do(s, "POST", "/api/compact", "Admin", ""); !strings.Contains(rec.Body.String(), `"oldFiles":2`) {
		t.Errorf("compact: %s", rec.Body)
	}
	rec := do(s, "GET", "/api/old-files", "Admin", "")
	var got struct {
		Files     []OldFile
		Remaining int
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 2 || got.Remaining != 1 {
		t.Errorf("old-files = %+v", got)
	}
}

func TestMeShowsTheCallersLabels(t *testing.T) {
	s, _ := newTestServer()
	rec := do(s, "GET", "/api/me", "Patient", "p-0001")
	var got struct {
		Role, Patient string
		Auths         []string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Role != "Patient" || got.Patient != "p-0001" || !slices.Equal(got.Auths, []string{"PATIENT_p0001"}) {
		t.Errorf("me = %+v", got)
	}
	if len(s.audit.Recent()) != 0 {
		t.Errorf("me was audited")
	}
}

func TestAuditKeepsLast100(t *testing.T) {
	a := &Audit{}
	for i := 0; i < 150; i++ {
		a.Add(Entry{Action: "read", Row: "p-0001"})
	}
	if n := len(a.Recent()); n != auditSize {
		t.Errorf("audit has %d entries, want %d", n, auditSize)
	}
}
