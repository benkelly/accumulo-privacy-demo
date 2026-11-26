package main

import (
	"slices"
	"testing"
)

func TestAuthsFor(t *testing.T) {
	tests := []struct {
		role, patient string
		want          []string
		wantErr       bool
	}{
		{role: "Clinician", want: []string{"PII", "CLINICIAN"}},
		{role: "Legal", want: []string{"PII", "LEGAL"}},
		{role: "Analyst", want: []string{}},
		{role: "Patient", patient: "p-0001", want: []string{"PATIENT_p0001"}},
		{role: "Patient", patient: "p-0020", want: []string{"PATIENT_p0020"}},
		{role: "Patient", patient: "", wantErr: true},
		{role: "Patient", patient: "p-0021", wantErr: true},
		{role: "Patient", patient: "p-0000", wantErr: true},
		{role: "Patient", patient: "PATIENT_p0001", wantErr: true},
		{role: "admin", wantErr: true},
		{role: "Root", wantErr: true},
	}
	for _, tt := range tests {
		got, err := authsFor(tt.role, tt.patient)
		if (err != nil) != tt.wantErr {
			t.Errorf("authsFor(%q, %q) error = %v, want error %v", tt.role, tt.patient, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && !slices.Equal(got, tt.want) {
			t.Errorf("authsFor(%q, %q) = %v, want %v", tt.role, tt.patient, got, tt.want)
		}
	}
}

func TestAdminHoldsEveryLabel(t *testing.T) {
	got, _ := authsFor("Admin", "")
	for _, label := range []string{"PII", "CLINICIAN", "LEGAL", "PATIENT_p0001", "PATIENT_p0020"} {
		if !slices.Contains(got, label) {
			t.Errorf("Admin is missing %s", label)
		}
	}
	if len(got) != 3+patientCount {
		t.Errorf("Admin has %d labels, want %d", len(got), 3+patientCount)
	}
}

func TestPatientLabelsDoNotLeakIntoOtherRoles(t *testing.T) {
	for _, role := range []string{"Clinician", "Legal", "Analyst"} {
		got, _ := authsFor(role, "p-0001")
		if slices.Contains(got, "PATIENT_p0001") {
			t.Errorf("%s got PATIENT_p0001 from the X-Demo-Patient header", role)
		}
	}
}
