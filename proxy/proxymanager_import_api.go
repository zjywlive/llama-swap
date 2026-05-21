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

const maxScannedGGUFModels = 200

type scanLocalModelsRequest struct {
	Dir       string `json:"dir"`
	Recursive bool   `json:"recursive"`
}

type scannedGGUFModel struct {
	Path         string         `json:"path"`
	Name         string         `json:"name"`
	IDSuggestion string         `json:"idSuggestion"`
	Imported     bool           `json:"imported"`
	ExistingID   string         `json:"existingId"`
	ModelInfo    *ggufModelInfo `json:"modelInfo,omitempty"`
	Warnings     []string       `json:"warnings"`
}

type scanLocalModelsResponse struct {
	Dir      string             `json:"dir"`
	Models   []scannedGGUFModel `json:"models"`
	Warnings []string           `json:"warnings"`
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
	files := []string{}

	if !stat.IsDir() {
		if isGGUFPath(dir) {
			files = append(files, dir)
		}
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
				return nil
			}
			if !isGGUFPath(path) {
				return nil
			}
			files = append(files, path)
			if len(files) >= maxScannedGGUFModels {
				warnings = append(warnings, fmt.Sprintf("scan stopped after %d GGUF files", maxScannedGGUFModels))
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
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			if isGGUFPath(path) {
				files = append(files, path)
			}
		}
	}

	sort.Strings(files)
	models := make([]scannedGGUFModel, 0, len(files))
	for _, path := range files {
		scanned := pm.scannedModelFromPath(path, importedPaths, usedIDs)
		models = append(models, scanned)
		usedIDs[scanned.IDSuggestion] = true
	}

	c.JSON(http.StatusOK, scanLocalModelsResponse{
		Dir:      dir,
		Models:   models,
		Warnings: warnings,
	})
}

func (pm *ProxyManager) scannedModelFromPath(path string, importedPaths map[string]string, usedIDs map[string]bool) scannedGGUFModel {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	baseID := modelIDFromPath(absPath)
	existingID := importedPaths[absPath]
	scanned := scannedGGUFModel{
		Path:         absPath,
		Name:         displayNameFromModelPath(absPath),
		IDSuggestion: uniqueModelID(baseID, usedIDs),
		Imported:     existingID != "",
		ExistingID:   existingID,
		Warnings:     []string{},
	}
	if existingID != "" {
		scanned.IDSuggestion = existingID
	}

	info, err := inspectGGUFModel(absPath)
	if err != nil {
		scanned.Warnings = []string{err.Error()}
		return scanned
	}
	scanned.ModelInfo = info
	if info.Name != "" {
		scanned.Name = info.Name
	}
	return scanned
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
