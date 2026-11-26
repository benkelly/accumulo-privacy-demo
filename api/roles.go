package main

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
)

// patientCount matches the seed (gateway/Seed.java) and the gateway's labels (accumulo/init.sh).
const patientCount = 20

// roleAuths is the one place that decides what each role can read.
// Patient is not here: its one label depends on the patient, see authsFor.
var roleAuths = map[string][]string{
	"Admin":     append([]string{"PII", "CLINICIAN", "LEGAL"}, allPatientLabels()...),
	"Clinician": {"PII", "CLINICIAN"},
	"Legal":     {"PII", "LEGAL"},
	"Analyst":   {},
}

// Row keys are opaque ids such as p-0001, never names or emails.
// Labels protect values, not row keys: anyone who can see one cell in a row also sees its key.
var patientID = regexp.MustCompile(`^p-(\d{4})$`)

// authsFor returns the Accumulo authorisations for a role. Patient needs a patient id.
func authsFor(role, patient string) ([]string, error) {
	if role == "Patient" {
		label, err := patientLabel(patient)
		if err != nil {
			return nil, err
		}
		return []string{label}, nil
	}
	auths, ok := roleAuths[role]
	if !ok {
		return nil, fmt.Errorf("unknown role %q: use Admin, Clinician, Legal, Analyst or Patient", role)
	}
	// A copy, so a caller that changes its slice cannot change the table for everyone.
	return slices.Clone(auths), nil
}

// patientLabel turns "p-0001" into "PATIENT_p0001". It rejects ids outside the seeded range.
func patientLabel(id string) (string, error) {
	if err := checkPatientID(id); err != nil {
		return "", err
	}
	return "PATIENT_p" + id[2:], nil
}

func checkPatientID(id string) error {
	m := patientID.FindStringSubmatch(id)
	if m == nil {
		return fmt.Errorf("patient id %q must look like p-0001", id)
	}
	if n, _ := strconv.Atoi(m[1]); n < 1 || n > patientCount {
		return fmt.Errorf("no patient %s: ids run from p-0001 to p-%04d", id, patientCount)
	}
	return nil
}

// allPatientLabels lists PATIENT_p0001 to PATIENT_p0020. Accumulo labels have no wildcards.
func allPatientLabels() []string {
	labels := make([]string, patientCount)
	for i := range labels {
		labels[i] = fmt.Sprintf("PATIENT_p%04d", i+1)
	}
	return labels
}
