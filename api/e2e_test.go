//go:build e2e

// End-to-end checks against the running stack. `make test` starts it and runs these.
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

var (
	apiURL = envOr("API_URL", "http://localhost:8080")
	webURL = envOr("WEB_URL", "http://localhost:5173")
)

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func call(t *testing.T, method, url, role, patient string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Demo-Role", role)
	if patient != "" {
		req.Header.Set("X-Demo-Patient", patient)
	}
	res, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, body
}

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("bad JSON %s: %v", body, err)
	}
	return v
}

func listAs(t *testing.T, role, patient string) map[string]map[string]string {
	t.Helper()
	code, body := call(t, "GET", apiURL+"/api/patients", role, patient)
	if code != http.StatusOK {
		t.Fatalf("%s list: %d %s", role, code, body)
	}
	out := map[string]map[string]string{}
	for _, p := range decode[[]Patient](t, body) {
		out[p.ID] = p.Cells
	}
	if len(out) != patientCount {
		t.Fatalf("%s sees %d patients, want %d. Run make seed?", role, len(out), patientCount)
	}
	return out
}

func TestE2EWhatEachRoleSees(t *testing.T) {
	public := []string{"info:county", "info:visits"}
	tests := []struct {
		role, patient string
		// columns the role must see on every row, and must never see on any row
		always, never []string
	}{
		{"Analyst", "", public, []string{"info:name", "clinical:diagnosis", "clinical:notes",
			"legal:case_note", "legal:medical_evidence", "legal:hold"}},
		{"Clinician", "", append([]string{"info:name", "clinical:diagnosis", "clinical:notes"}, public...),
			[]string{"legal:case_note", "legal:medical_evidence", "legal:hold"}},
		{"Legal", "", append([]string{"info:name", "legal:hold"}, public...),
			[]string{"clinical:diagnosis", "clinical:notes", "legal:medical_evidence"}},
		{"Admin", "", append([]string{"info:name", "clinical:diagnosis", "clinical:notes", "legal:hold"}, public...), nil},
	}
	for _, tt := range tests {
		for id, cells := range listAs(t, tt.role, tt.patient) {
			for _, col := range tt.always {
				if _, ok := cells[col]; !ok {
					t.Errorf("%s does not see %s on %s", tt.role, col, id)
				}
			}
			for _, col := range tt.never {
				if _, ok := cells[col]; ok {
					t.Errorf("%s sees %s on %s", tt.role, col, id)
				}
			}
		}
	}
}

func TestE2ELegalSeesCaseNotesButNotMedicalEvidence(t *testing.T) {
	legal, admin := listAs(t, "Legal", ""), listAs(t, "Admin", "")
	for id, cells := range admin {
		_, has := cells["legal:case_note"]
		if _, sees := legal[id]["legal:case_note"]; sees != has {
			t.Errorf("Legal sees case note on %s: %v, stored: %v", id, sees, has)
		}
	}
	if _, ok := admin["p-0001"]["legal:medical_evidence"]; !ok {
		t.Errorf("Admin does not see medical evidence on p-0001")
	}
}

func TestE2EPatientSeesOnlyOwnDiagnosis(t *testing.T) {
	for id, cells := range listAs(t, "Patient", "p-0001") {
		_, ok := cells["clinical:diagnosis"]
		if ok != (id == "p-0001") {
			t.Errorf("Patient p-0001 sees a diagnosis on %s: %v", id, ok)
		}
		if _, ok := cells["info:name"]; ok {
			t.Errorf("Patient p-0001 sees a name on %s", id)
		}
	}
}

func TestE2EAdminOnly(t *testing.T) {
	for _, role := range []string{"Clinician", "Legal", "Analyst"} {
		for _, e := range [][2]string{{"DELETE", "/api/patients/p-0002"}, {"GET", "/api/patients/p-0002/raw"},
			{"POST", "/api/compact"}, {"GET", "/api/old-files"}, {"GET", "/api/audit"}} {
			if code, _ := call(t, e[0], apiURL+e[1], role, ""); code != http.StatusForbidden {
				t.Errorf("%s %s as %s: %d, want 403", e[0], e[1], role, code)
			}
		}
	}
}

func TestE2ELegalHoldRefusesDelete(t *testing.T) {
	for _, id := range []string{"p-0009", "p-0011"} {
		code, body := call(t, "DELETE", apiURL+"/api/patients/"+id, "Admin", "")
		if code != http.StatusConflict || !strings.Contains(string(body), "legal hold") {
			t.Errorf("delete %s: %d %s, want 409 with a legal hold message", id, code, body)
		}
		if code, _ := call(t, "GET", apiURL+"/api/patients/"+id, "Admin", ""); code != http.StatusOK {
			t.Errorf("read %s after refused delete: %d, want 200", id, code)
		}
	}
	_, body := call(t, "GET", apiURL+"/api/audit", "Admin", "")
	if !slices.ContainsFunc(decode[[]Entry](t, body), func(e Entry) bool {
		return e.Action == "delete-refused" && e.Row == "p-0009"
	}) {
		t.Errorf("audit has no delete-refused entry for p-0009")
	}
}

// The 4 steps of the delete flow on p-0005. `make test` seeds again afterwards.
func TestE2EDeleteInFourSteps(t *testing.T) {
	const id = "p-0005"
	counts := func() (cells, markers int) {
		_, body := call(t, "GET", apiURL+"/api/patients/"+id+"/raw", "Admin", "")
		for _, c := range decode[[]Cell](t, body) {
			if c.Deleted {
				markers++
			} else {
				cells++
			}
		}
		return
	}

	// 1. Delete written: reads return nothing.
	if code, body := call(t, "DELETE", apiURL+"/api/patients/"+id, "Admin", ""); code != http.StatusOK {
		t.Fatalf("delete: %d %s", code, body)
	}
	if code, _ := call(t, "GET", apiURL+"/api/patients/"+id, "Admin", ""); code != http.StatusNotFound {
		t.Fatalf("read after delete: %d, want 404", code)
	}

	// 2. Still on disk: the files hold the cells and the delete markers.
	if cells, markers := counts(); cells == 0 || markers == 0 {
		t.Fatalf("after delete, files hold %d cells and %d markers, want both > 0", cells, markers)
	}

	// 3. Compaction: gone from the files in use, but old files are left on HDFS.
	code, body := call(t, "POST", apiURL+"/api/compact", "Admin", "")
	if code != http.StatusOK || decode[map[string]int](t, body)["oldFiles"] == 0 {
		t.Fatalf("compact: %d %s, want some old files", code, body)
	}
	if cells, markers := counts(); cells+markers != 0 {
		t.Fatalf("after compaction, files hold %d cells and %d markers, want none", cells, markers)
	}

	// 4. Garbage collection: the old files leave HDFS. The demo's collector runs every 30s.
	deadline := time.Now().Add(2 * time.Minute)
	for {
		_, body := call(t, "GET", apiURL+"/api/old-files", "Admin", "")
		if decode[struct{ Remaining int }](t, body).Remaining == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("old files still on HDFS after 2 minutes: %s", body)
		}
		time.Sleep(2 * time.Second)
	}
}

func TestE2EWebServesThePageAndForwardsTheAPI(t *testing.T) {
	code, body := call(t, "GET", webURL+"/", "", "")
	if code != http.StatusOK || !strings.Contains(string(body), `id="root"`) {
		t.Errorf("web page: %d", code)
	}
	code, body = call(t, "GET", webURL+"/api/me", "Legal", "")
	if code != http.StatusOK || !slices.Equal(decode[struct{ Auths []string }](t, body).Auths, []string{"PII", "LEGAL"}) {
		t.Errorf("web /api/me as Legal: %d %s", code, body)
	}
}
