package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Cell is one Accumulo cell, as the gateway returns it.
type Cell struct {
	Row        string `json:"row"`
	Family     string `json:"family"`
	Qualifier  string `json:"qualifier"`
	Visibility string `json:"visibility"`
	Value      string `json:"value"`
	Deleted    bool   `json:"deleted"`
}

// Store is what the handlers need from the gateway. Tests use a fake.
type Store interface {
	Scan(row string, auths []string) ([]Cell, error) // row "" means every row
	Delete(row string) (int, error)
	Compact() (int, error) // returns how many files it replaced
	OldFiles() ([]OldFile, error)
	Raw(row string) ([]Cell, error)
}

// OldFile is a file that the last compaction replaced. It stays on HDFS until garbage collection.
type OldFile struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

// Gateway calls the Java gateway over HTTP. See gateway/src/main/java/gateway/Gateway.java.
type Gateway struct {
	base string
	http *http.Client
}

func NewGateway(base string) *Gateway {
	// Compaction waits until it is done, so allow more time than a read needs.
	return &Gateway{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 2 * time.Minute}}
}

func (g *Gateway) Scan(row string, auths []string) ([]Cell, error) {
	q := url.Values{"auths": {strings.Join(auths, ",")}}
	if row != "" {
		q.Set("row", row)
	}
	var cells []Cell
	return cells, g.call(http.MethodGet, "/scan?"+q.Encode(), nil, &cells)
}

func (g *Gateway) Delete(row string) (int, error) {
	var out struct{ Deleted int }
	err := g.call(http.MethodPost, "/delete", map[string]string{"row": row}, &out)
	return out.Deleted, err
}

func (g *Gateway) Compact() (int, error) {
	var out struct{ OldFiles int }
	err := g.call(http.MethodPost, "/compact", nil, &out)
	return out.OldFiles, err
}

func (g *Gateway) OldFiles() ([]OldFile, error) {
	var files []OldFile
	return files, g.call(http.MethodGet, "/old-files", nil, &files)
}

func (g *Gateway) Raw(row string) ([]Cell, error) {
	var cells []Cell
	return cells, g.call(http.MethodGet, "/raw?"+url.Values{"row": {row}}.Encode(), nil, &cells)
}

func (g *Gateway) call(method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, g.base+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("gateway: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		var e struct{ Error string }
		_ = json.NewDecoder(res.Body).Decode(&e)
		return fmt.Errorf("gateway %s %s: %d %s", method, path, res.StatusCode, e.Error)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}
