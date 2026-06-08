package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fox-in-the-box-ai/fox-fleet/skillsets"
)

const maxSkillsetUpload = 1 << 20 // 1 MB

type skillsetSummary struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ContractVersion string `json:"contract_version"`
	ToolCount       int    `json:"tool_count"`
	SourceCount     int    `json:"source_count"`
	Filename        string `json:"filename"`
}

func (s *Server) handleListSkillsets(w http.ResponseWriter, _ *http.Request) {
	entries, err := os.ReadDir(s.skillsetsDir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot read skillsets directory")
		return
	}

	var items []skillsetSummary
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.skillsetsDir, e.Name()))
		if err != nil {
			s.log.Warn("failed to read skillset file", "file", e.Name(), "error", err)
			continue
		}
		m, err := skillsets.Parse(data)
		if err != nil {
			s.log.Warn("failed to parse skillset file", "file", e.Name(), "error", err)
			continue
		}
		items = append(items, skillsetSummary{
			Name:            m.Name,
			Version:         m.Version,
			ContractVersion: m.ContractVersion,
			ToolCount:       len(m.Tools),
			SourceCount:     len(m.DataSources),
			Filename:        e.Name(),
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	if items == nil {
		items = []skillsetSummary{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleGetSkillset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validInstanceID.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid skillset name")
		return
	}
	path := filepath.Join(s.skillsetsDir, name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			path = filepath.Join(s.skillsetsDir, name+".yml")
			data, err = os.ReadFile(path)
		}
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("skillset %s not found", name))
			return
		}
	}
	m, err := skillsets.Parse(data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "parse_error",
			fmt.Sprintf("skillset %q is stored on disk but cannot be parsed — re-upload or check YAML syntax", name))
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleUploadSkillset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSkillsetUpload)

	if err := r.ParseMultipartForm(maxSkillsetUpload); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "request is not valid multipart form data or exceeds 1 MB limit")
		return
	}

	f, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "multipart form must include a \"file\" field")
		return
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxSkillsetUpload))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot read uploaded file")
		return
	}

	m, err := skillsets.Parse(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if !validInstanceID.MatchString(m.Name) {
		writeError(w, http.StatusBadRequest, "validation_error",
			"skillset name must be alphanumeric with dots/hyphens/underscores (max 64 chars)")
		return
	}

	if err := os.MkdirAll(s.skillsetsDir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot create skillsets directory")
		return
	}

	dest := filepath.Join(s.skillsetsDir, m.Name+".yaml")
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot save skillset to disk")
		return
	}

	writeJSON(w, http.StatusCreated, skillsetSummary{
		Name:            m.Name,
		Version:         m.Version,
		ContractVersion: m.ContractVersion,
		ToolCount:       len(m.Tools),
		SourceCount:     len(m.DataSources),
		Filename:        m.Name + ".yaml",
	})
}

func (s *Server) handleDownloadSkillset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validInstanceID.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid skillset name")
		return
	}
	path := filepath.Join(s.skillsetsDir, name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			path = filepath.Join(s.skillsetsDir, name+".yml")
			data, err = os.ReadFile(path)
		}
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("skillset %s not found", name))
			return
		}
	}
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+".yaml"))
	_, _ = w.Write(data)
}

func (s *Server) handleDeleteSkillset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validInstanceID.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid skillset name")
		return
	}
	path := filepath.Join(s.skillsetsDir, name+".yaml")
	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			path = filepath.Join(s.skillsetsDir, name+".yml")
			err = os.Remove(path)
		}
		if err != nil {
			if os.IsNotExist(err) {
				writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("skillset %s not found", name))
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "cannot delete skillset from disk")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
