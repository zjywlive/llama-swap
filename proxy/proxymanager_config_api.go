package proxy

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mostlygeek/llama-swap/proxy/config"
	"gopkg.in/yaml.v3"
)

type editableTimeoutsConfig struct {
	Connect        int `json:"connect"`
	KeepAlive      int `json:"keepalive"`
	ResponseHeader int `json:"responseHeader"`
	TLSHandshake   int `json:"tlsHandshake"`
	ExpectContinue int `json:"expectContinue"`
	IdleConn       int `json:"idleConn"`
}

type editableModelFilters struct {
	StripParams   string                    `json:"stripParams"`
	SetParams     map[string]any            `json:"setParams"`
	SetParamsByID map[string]map[string]any `json:"setParamsByID"`
}

type editableModelConfig struct {
	ID               string                 `json:"id"`
	Cmd              string                 `json:"cmd"`
	CmdStop          string                 `json:"cmdStop"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	Env              []string               `json:"env"`
	Proxy            string                 `json:"proxy"`
	Aliases          []string               `json:"aliases"`
	CheckEndpoint    string                 `json:"checkEndpoint"`
	TTL              int                    `json:"ttl"`
	Unlisted         bool                   `json:"unlisted"`
	UseModelName     string                 `json:"useModelName"`
	ConcurrencyLimit int                    `json:"concurrencyLimit"`
	SendLoadingState *bool                  `json:"sendLoadingState"`
	Filters          editableModelFilters   `json:"filters"`
	Metadata         map[string]any         `json:"metadata"`
	Timeouts         editableTimeoutsConfig `json:"timeouts"`
	ModelInfo        *ggufModelInfo         `json:"modelInfo,omitempty"`
}

type editableModelsResponse struct {
	ConfigPath     string                `json:"configPath"`
	EditingEnabled bool                  `json:"editingEnabled"`
	Models         []editableModelConfig `json:"models"`
}

type inspectModelRequest struct {
	Path string `json:"path"`
}

func (pm *ProxyManager) requireConfigEditing(c *gin.Context) bool {
	if !pm.configEditingEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "config editing is disabled; start llama-swap with --allow-config-edit"})
		return false
	}
	if strings.TrimSpace(pm.configPath) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config path is not available"})
		return false
	}
	return true
}

func editableFromModelConfig(id string, mc config.ModelConfig) editableModelConfig {
	filters := editableModelFilters{
		StripParams:   mc.Filters.StripParams,
		SetParams:     map[string]any{},
		SetParamsByID: map[string]map[string]any{},
	}
	if mc.Filters.SetParams != nil {
		filters.SetParams = mc.Filters.SetParams
	}
	if mc.Filters.SetParamsByID != nil {
		filters.SetParamsByID = mc.Filters.SetParamsByID
	}

	metadata := map[string]any{}
	if mc.Metadata != nil {
		metadata = mc.Metadata
	}

	return editableModelConfig{
		ID:               id,
		Cmd:              mc.Cmd,
		CmdStop:          mc.CmdStop,
		Name:             mc.Name,
		Description:      mc.Description,
		Env:              mc.Env,
		Proxy:            mc.Proxy,
		Aliases:          mc.Aliases,
		CheckEndpoint:    mc.CheckEndpoint,
		TTL:              mc.UnloadAfter,
		Unlisted:         mc.Unlisted,
		UseModelName:     mc.UseModelName,
		ConcurrencyLimit: mc.ConcurrencyLimit,
		SendLoadingState: mc.SendLoadingState,
		Filters:          filters,
		Metadata:         metadata,
		Timeouts: editableTimeoutsConfig{
			Connect:        mc.Timeouts.Connect,
			KeepAlive:      mc.Timeouts.KeepAlive,
			ResponseHeader: mc.Timeouts.ResponseHeader,
			TLSHandshake:   mc.Timeouts.TLSHandshake,
			ExpectContinue: mc.Timeouts.ExpectContinue,
			IdleConn:       mc.Timeouts.IdleConn,
		},
	}
}

func (pm *ProxyManager) apiGetEditableModels(c *gin.Context) {
	if !pm.requireConfigEditing(c) {
		return
	}

	modelIDs := make([]string, 0, len(pm.config.Models))
	for modelID := range pm.config.Models {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)

	models := make([]editableModelConfig, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		editable, err := pm.editableModelFromRawConfig(modelID)
		if err != nil {
			pm.proxyLogger.Warnf("Unable to read raw model config for %s: %v", modelID, err)
			editable = editableFromModelConfig(modelID, pm.config.Models[modelID])
		}
		models = append(models, editable)
	}

	configPath, _ := filepath.Abs(pm.configPath)
	c.JSON(http.StatusOK, editableModelsResponse{
		ConfigPath:     configPath,
		EditingEnabled: true,
		Models:         models,
	})
}

func (pm *ProxyManager) apiGetEditableModel(c *gin.Context) {
	if !pm.requireConfigEditing(c) {
		return
	}

	requestedModel := strings.TrimPrefix(c.Param("model"), "/")
	realModelName, found := pm.config.RealModelName(requestedModel)
	if !found {
		pm.sendErrorResponse(c, http.StatusNotFound, "Model not found")
		return
	}

	editable, err := pm.editableModelFromRawConfig(realModelName)
	if err != nil {
		pm.proxyLogger.Warnf("Unable to read raw model config for %s: %v", realModelName, err)
		editable = editableFromModelConfig(realModelName, pm.config.Models[realModelName])
	}
	c.JSON(http.StatusOK, editable)
}

func (pm *ProxyManager) editableModelFromRawConfig(modelID string) (editableModelConfig, error) {
	source, err := os.ReadFile(pm.configPath)
	if err != nil {
		return editableModelConfig{}, fmt.Errorf("read config: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(source, &root); err != nil {
		return editableModelConfig{}, fmt.Errorf("parse config: %w", err)
	}
	rootMap, err := documentMapping(&root)
	if err != nil {
		return editableModelConfig{}, err
	}
	modelsNode := mappingValue(rootMap, "models")
	if modelsNode == nil || modelsNode.Kind != yaml.MappingNode {
		return editableModelConfig{}, fmt.Errorf("models must be a mapping")
	}
	modelNode := mappingValue(modelsNode, modelID)
	if modelNode == nil {
		return editableModelConfig{}, fmt.Errorf("model %s not found", modelID)
	}

	var modelConfig config.ModelConfig
	if err := modelNode.Decode(&modelConfig); err != nil {
		return editableModelConfig{}, fmt.Errorf("decode model %s: %w", modelID, err)
	}
	editable := editableFromModelConfig(modelID, modelConfig)
	editable.ModelInfo = pm.inspectModelForEditableConfig(editable)
	return editable, nil
}

func (pm *ProxyManager) inspectModelForEditableConfig(model editableModelConfig) *ggufModelInfo {
	modelPath, ok := modelPathFromCommand(model.Cmd)
	if !ok {
		return nil
	}
	info, err := inspectGGUFModel(modelPath)
	if err != nil {
		return &ggufModelInfo{
			Path:     modelPath,
			Warnings: []string{err.Error()},
		}
	}
	return info
}

func (pm *ProxyManager) apiInspectEditableModel(c *gin.Context) {
	if !pm.requireConfigEditing(c) {
		return
	}

	var request inspectModelRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	modelPath := strings.TrimSpace(request.Path)
	if modelPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model path is required"})
		return
	}

	info, err := inspectGGUFModel(modelPath)
	if err != nil {
		c.JSON(http.StatusOK, ggufModelInfo{
			Path:     modelPath,
			Warnings: []string{err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, info)
}

func (pm *ProxyManager) apiValidateEditableModel(c *gin.Context) {
	if !pm.requireConfigEditing(c) {
		return
	}

	var model editableModelConfig
	if err := c.ShouldBindJSON(&model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.ID = strings.TrimSpace(model.ID)
	if model.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model id is required"})
		return
	}

	if _, err := pm.renderConfigWithModel(model.ID, model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "ok"})
}

func (pm *ProxyManager) apiSaveEditableModel(c *gin.Context) {
	if !pm.requireConfigEditing(c) {
		return
	}

	requestedModel := strings.TrimPrefix(c.Param("model"), "/")
	realModelName, found := pm.config.RealModelName(requestedModel)
	if !found {
		pm.sendErrorResponse(c, http.StatusNotFound, "Model not found")
		return
	}

	var model editableModelConfig
	if err := c.ShouldBindJSON(&model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model.ID = realModelName

	pm.Lock()
	defer pm.Unlock()

	rendered, err := pm.renderConfigWithModel(realModelName, model)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stat, err := os.Stat(pm.configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("stat config: %v", err)})
		return
	}

	backupPath := pm.configPath + ".bak"
	if existing, err := os.ReadFile(pm.configPath); err == nil {
		if err := os.WriteFile(backupPath, existing, stat.Mode()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("write backup: %v", err)})
			return
		}
	}

	tmpPath := pm.configPath + ".tmp"
	if err := os.WriteFile(tmpPath, rendered, stat.Mode()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("write temp config: %v", err)})
		return
	}
	if err := os.Rename(tmpPath, pm.configPath); err != nil {
		_ = os.Remove(tmpPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("replace config: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"msg":            "ok",
		"backupPath":     backupPath,
		"requiresReload": true,
	})
}

func (pm *ProxyManager) renderConfigWithModel(modelID string, model editableModelConfig) ([]byte, error) {
	if strings.TrimSpace(model.Cmd) == "" {
		return nil, fmt.Errorf("cmd is required")
	}
	if err := validateGGUFRuntimeLimits(model.Cmd); err != nil {
		return nil, err
	}

	source, err := os.ReadFile(pm.configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(source, &root); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	rootMap, err := documentMapping(&root)
	if err != nil {
		return nil, err
	}

	modelsNode := mappingValue(rootMap, "models")
	if modelsNode == nil {
		modelsNode = &yaml.Node{Kind: yaml.MappingNode}
		setMappingValue(rootMap, "models", modelsNode)
	}
	if modelsNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("models must be a mapping")
	}

	previousModelNode := mappingValue(modelsNode, modelID)
	modelNode, err := editableModelToYAMLNode(model)
	if err != nil {
		return nil, err
	}
	preserveModelMacros(previousModelNode, modelNode)
	setMappingValue(modelsNode, modelID, modelNode)

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		_ = encoder.Close()
		return nil, fmt.Errorf("encode config: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close encoder: %w", err)
	}

	if _, err := config.LoadConfigFromReader(bytes.NewReader(out.Bytes())); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	return out.Bytes(), nil
}

func documentMapping(root *yaml.Node) (*yaml.Node, error) {
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			root.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
		}
		if root.Content[0].Kind != yaml.MappingNode {
			return nil, fmt.Errorf("config root must be a mapping")
		}
		return root.Content[0], nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config root must be a mapping")
	}
	return root, nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func setMappingValue(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

func preserveModelMacros(previous, next *yaml.Node) {
	if previous == nil || next == nil || previous.Kind != yaml.MappingNode || next.Kind != yaml.MappingNode {
		return
	}
	if mappingValue(next, "macros") != nil {
		return
	}
	if macros := mappingValue(previous, "macros"); macros != nil {
		setMappingValue(next, "macros", macros)
	}
}

func editableModelToYAMLNode(model editableModelConfig) (*yaml.Node, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}

	addStringNode(node, "name", model.Name, false)
	addStringNode(node, "description", model.Description, false)
	addStringSliceNode(node, "aliases", model.Aliases)
	addStringNode(node, "cmd", model.Cmd, true)
	addStringNode(node, "cmdStop", model.CmdStop, false)
	addStringSliceNode(node, "env", model.Env)
	addStringNode(node, "proxy", model.Proxy, false)
	addStringNode(node, "checkEndpoint", model.CheckEndpoint, false)
	addIntNode(node, "ttl", model.TTL)
	addBoolNode(node, "unlisted", model.Unlisted, false)
	addStringNode(node, "useModelName", model.UseModelName, false)
	addIntNodeIfNonZero(node, "concurrencyLimit", model.ConcurrencyLimit)
	if model.SendLoadingState != nil {
		addBoolNode(node, "sendLoadingState", *model.SendLoadingState, true)
	}
	addFiltersNode(node, model.Filters)
	addAnyMapNode(node, "metadata", model.Metadata)
	addTimeoutsNode(node, model.Timeouts)

	return node, nil
}

func addStringNode(mapping *yaml.Node, key, value string, required bool) {
	if !required && strings.TrimSpace(value) == "" {
		return
	}
	style := yaml.Style(0)
	if strings.Contains(value, "\n") {
		style = yaml.LiteralStyle
	}
	setMappingValue(mapping, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, Style: style})
}

func addIntNode(mapping *yaml.Node, key string, value int) {
	setMappingValue(mapping, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprintf("%d", value)})
}

func addIntNodeIfNonZero(mapping *yaml.Node, key string, value int) {
	if value == 0 {
		return
	}
	addIntNode(mapping, key, value)
}

func addBoolNode(mapping *yaml.Node, key string, value bool, always bool) {
	if !always && !value {
		return
	}
	setMappingValue(mapping, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: fmt.Sprintf("%t", value)})
}

func addStringSliceNode(mapping *yaml.Node, key string, values []string) {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	if len(clean) == 0 {
		return
	}

	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, value := range clean {
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
	}
	setMappingValue(mapping, key, seq)
}

func addFiltersNode(mapping *yaml.Node, filters editableModelFilters) {
	filtersNode := &yaml.Node{Kind: yaml.MappingNode}
	addStringNode(filtersNode, "stripParams", filters.StripParams, false)
	addAnyMapNode(filtersNode, "setParams", filters.SetParams)
	addAnyMapNode(filtersNode, "setParamsByID", mapStringMapToAny(filters.SetParamsByID))
	if len(filtersNode.Content) > 0 {
		setMappingValue(mapping, "filters", filtersNode)
	}
}

func addTimeoutsNode(mapping *yaml.Node, timeouts editableTimeoutsConfig) {
	if timeouts.Connect == 30 &&
		timeouts.KeepAlive == 30 &&
		timeouts.ResponseHeader == 0 &&
		timeouts.TLSHandshake == 10 &&
		timeouts.ExpectContinue == 1 &&
		timeouts.IdleConn == 90 {
		return
	}

	timeoutsNode := &yaml.Node{Kind: yaml.MappingNode}
	addIntNode(timeoutsNode, "connect", timeouts.Connect)
	addIntNode(timeoutsNode, "keepalive", timeouts.KeepAlive)
	addIntNode(timeoutsNode, "responseHeader", timeouts.ResponseHeader)
	addIntNode(timeoutsNode, "tlsHandshake", timeouts.TLSHandshake)
	addIntNode(timeoutsNode, "expectContinue", timeouts.ExpectContinue)
	addIntNode(timeoutsNode, "idleConn", timeouts.IdleConn)
	setMappingValue(mapping, "timeouts", timeoutsNode)
}

func addAnyMapNode(mapping *yaml.Node, key string, value map[string]any) {
	if len(value) == 0 {
		return
	}
	var valueNode yaml.Node
	if err := valueNode.Encode(value); err != nil {
		return
	}
	setMappingValue(mapping, key, &valueNode)
}

func mapStringMapToAny(value map[string]map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, nested := range value {
		out[key] = nested
	}
	return out
}
