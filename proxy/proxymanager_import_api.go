package proxy

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

const maxScannedLocalModels = 200

type scanLocalModelsRequest struct {
	Dir       string `json:"dir"`
	Recursive bool   `json:"recursive"`
}

type localModelCandidate struct {
	Path    string
	Backend string
}

type scannedLocalModel struct {
	Path         string         `json:"path"`
	Name         string         `json:"name"`
	Backend      string         `json:"backend"`
	Format       string         `json:"format"`
	IDSuggestion string         `json:"idSuggestion"`
	Imported     bool           `json:"imported"`
	ExistingID   string         `json:"existingId"`
	ModelInfo    *ggufModelInfo `json:"modelInfo,omitempty"`
	Warnings     []string       `json:"warnings"`
}

type scanLocalModelsResponse struct {
	Dir      string              `json:"dir"`
	Models   []scannedLocalModel `json:"models"`
	Warnings []string            `json:"warnings"`
}

func (pm *ProxyManager) apiScanLocalModels(c *gin.Context) {
	if !pm.requireConfigEditing(c) {
		return
	}

	var request scanLocalModelsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dir := expandLocalModelPath(request.Dir)
	if strings.TrimSpace(dir) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "directory is required"})
		return
	}

	stat, err := os.Stat(dir)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("stat directory: %v", err)})
		return
	}

	importedPaths := pm.importedModelPaths()
	usedIDs := pm.usedModelIDs()
	warnings := []string{}
	candidates := []localModelCandidate{}
	seen := map[string]bool{}
	addCandidate := func(path, backend string) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}
		key := backend + "\x00" + absPath
		if seen[key] {
			return
		}
		seen[key] = true
		candidates = append(candidates, localModelCandidate{Path: absPath, Backend: backend})
	}

	if !stat.IsDir() {
		if isGGUFPath(dir) {
			addCandidate(dir, "llama-server")
		}
	} else if looksLikeMLXModelDir(dir) {
		addCandidate(dir, "mlx-lm")
	} else if request.Recursive {
		err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				warnings = append(warnings, walkErr.Error())
				return nil
			}
			if entry.IsDir() {
				if entry.Name() != "." && strings.HasPrefix(entry.Name(), ".") {
					return filepath.SkipDir
				}
				if path != dir && looksLikeMLXModelDir(path) {
					addCandidate(path, "mlx-lm")
					if len(candidates) >= maxScannedLocalModels {
						warnings = append(warnings, fmt.Sprintf("scan stopped after %d local models", maxScannedLocalModels))
						return filepath.SkipAll
					}
					return filepath.SkipDir
				}
				return nil
			}
			if !isGGUFPath(path) {
				return nil
			}
			addCandidate(path, "llama-server")
			if len(candidates) >= maxScannedLocalModels {
				warnings = append(warnings, fmt.Sprintf("scan stopped after %d local models", maxScannedLocalModels))
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("scan directory: %v", err)})
			return
		}
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("read directory: %v", err)})
			return
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				if looksLikeMLXModelDir(path) {
					addCandidate(path, "mlx-lm")
				}
				continue
			}
			if isGGUFPath(path) {
				addCandidate(path, "llama-server")
			}
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Path < candidates[j].Path
	})
	models := make([]scannedLocalModel, 0, len(candidates))
	for _, candidate := range candidates {
		scanned := pm.scannedModelFromCandidate(candidate, importedPaths, usedIDs)
		models = append(models, scanned)
		usedIDs[scanned.IDSuggestion] = true
	}

	c.JSON(http.StatusOK, scanLocalModelsResponse{
		Dir:      dir,
		Models:   models,
		Warnings: warnings,
	})
}

func (pm *ProxyManager) scannedModelFromCandidate(candidate localModelCandidate, importedPaths map[string]string, usedIDs map[string]bool) scannedLocalModel {
	absPath, err := filepath.Abs(candidate.Path)
	if err != nil {
		absPath = candidate.Path
	}

	baseID := modelIDFromPath(absPath)
	existingID := importedPaths[absPath]
	scanned := scannedLocalModel{
		Path:         absPath,
		Name:         displayNameFromModelPath(absPath),
		Backend:      candidate.Backend,
		Format:       scannedFormatForBackend(candidate.Backend),
		IDSuggestion: uniqueModelID(baseID, usedIDs),
		Imported:     existingID != "",
		ExistingID:   existingID,
		Warnings:     []string{},
	}
	if existingID != "" {
		scanned.IDSuggestion = existingID
	}

	info, err := inspectLocalModelPath(absPath)
	if err != nil {
		scanned.Warnings = []string{err.Error()}
		return scanned
	}
	scanned.ModelInfo = info
	scanned.Backend = info.Backend
	scanned.Format = info.Format
	if info.Name != "" {
		scanned.Name = info.Name
	}
	return scanned
}

func scannedFormatForBackend(backend string) string {
	if backend == "mlx-lm" {
		return "mlx"
	}
	return "gguf"
}

func (pm *ProxyManager) importedModelPaths() map[string]string {
	out := map[string]string{}
	for id, model := range pm.config.Models {
		modelPath, ok := modelPathFromCommand(model.Cmd)
		if !ok || modelPath == "" {
			continue
		}
		expanded := expandLocalModelPath(modelPath)
		absPath, err := filepath.Abs(expanded)
		if err != nil {
			absPath = expanded
		}
		out[absPath] = id
	}
	return out
}

func (pm *ProxyManager) usedModelIDs() map[string]bool {
	out := map[string]bool{}
	for id, model := range pm.config.Models {
		out[id] = true
		for _, alias := range model.Aliases {
			out[alias] = true
		}
	}
	return out
}

func isGGUFPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".gguf")
}

func displayNameFromModelPath(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	return strings.TrimSpace(name)
}

func modelIDFromPath(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name = strings.ToLower(name)
	re := regexp.MustCompile(`[^a-z0-9._-]+`)
	name = re.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-._")
	if name == "" {
		return "local-model"
	}
	return name
}

func uniqueModelID(base string, used map[string]bool) string {
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !used[candidate] {
			return candidate
		}
	}
}
