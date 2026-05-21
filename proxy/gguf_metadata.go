package proxy

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/mostlygeek/llama-swap/proxy/config"
)

const (
	ggufTypeUint8   uint32 = 0
	ggufTypeInt8    uint32 = 1
	ggufTypeUint16  uint32 = 2
	ggufTypeInt16   uint32 = 3
	ggufTypeUint32  uint32 = 4
	ggufTypeInt32   uint32 = 5
	ggufTypeFloat32 uint32 = 6
	ggufTypeBool    uint32 = 7
	ggufTypeString  uint32 = 8
	ggufTypeArray   uint32 = 9
	ggufTypeUint64  uint32 = 10
	ggufTypeInt64   uint32 = 11
	ggufTypeFloat64 uint32 = 12

	maxGGUFStringLength = 16 * 1024 * 1024
)

type ggufModelLimits struct {
	ContextMax    int `json:"contextMax"`
	GPULayerMax   int `json:"gpuLayerMax"`
	BatchMax      int `json:"batchMax"`
	MicroBatchMax int `json:"microBatchMax"`
	ThreadsMax    int `json:"threadsMax"`
	ParallelMax   int `json:"parallelMax"`
}

type ggufModelInfo struct {
	Path              string          `json:"path"`
	Exists            bool            `json:"exists"`
	Format            string          `json:"format"`
	Version           uint32          `json:"version"`
	Name              string          `json:"name"`
	Architecture      string          `json:"architecture"`
	Quantization      string          `json:"quantization"`
	FileType          int             `json:"fileType"`
	ContextLength     int             `json:"contextLength"`
	BlockCount        int             `json:"blockCount"`
	EmbeddingLength   int             `json:"embeddingLength"`
	FeedForwardLength int             `json:"feedForwardLength"`
	HeadCount         int             `json:"headCount"`
	HeadCountKV       int             `json:"headCountKV"`
	VocabularySize    int             `json:"vocabularySize"`
	Limits            ggufModelLimits `json:"limits"`
	Warnings          []string        `json:"warnings"`
}

func inspectGGUFModel(modelPath string) (*ggufModelInfo, error) {
	expandedPath := expandLocalModelPath(modelPath)
	file, err := os.Open(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open model file: %w", err)
	}
	defer file.Close()

	metadata, version, err := readGGUFMetadata(file)
	if err != nil {
		return nil, err
	}

	info := ggufInfoFromMetadata(expandedPath, version, metadata)
	return &info, nil
}

func expandLocalModelPath(modelPath string) string {
	path := strings.TrimSpace(modelPath)
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return os.ExpandEnv(path)
}

func readGGUFMetadata(reader io.ReadSeeker) (map[string]any, uint32, error) {
	magic := make([]byte, 4)
	if _, err := io.ReadFull(reader, magic); err != nil {
		return nil, 0, fmt.Errorf("read GGUF magic: %w", err)
	}
	if string(magic) != "GGUF" {
		return nil, 0, fmt.Errorf("not a GGUF file")
	}

	version, err := readUint32(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("read GGUF version: %w", err)
	}
	if version < 2 || version > 3 {
		return nil, 0, fmt.Errorf("unsupported GGUF version %d", version)
	}

	if _, err := readUint64(reader); err != nil {
		return nil, 0, fmt.Errorf("read tensor count: %w", err)
	}
	metadataCount, err := readUint64(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("read metadata count: %w", err)
	}

	metadata := make(map[string]any)
	for i := uint64(0); i < metadataCount; i++ {
		key, err := readGGUFString(reader)
		if err != nil {
			return nil, 0, fmt.Errorf("read metadata key %d: %w", i, err)
		}
		valueType, err := readUint32(reader)
		if err != nil {
			return nil, 0, fmt.Errorf("read metadata value type for %s: %w", key, err)
		}
		value, keep, err := readGGUFValue(reader, valueType)
		if err != nil {
			return nil, 0, fmt.Errorf("read metadata value for %s: %w", key, err)
		}
		if keep {
			metadata[key] = value
		}
	}
	return metadata, version, nil
}

func readGGUFValue(reader io.ReadSeeker, valueType uint32) (any, bool, error) {
	switch valueType {
	case ggufTypeUint8:
		var value uint8
		err := binary.Read(reader, binary.LittleEndian, &value)
		return value, true, err
	case ggufTypeInt8:
		var value int8
		err := binary.Read(reader, binary.LittleEndian, &value)
		return value, true, err
	case ggufTypeUint16:
		var value uint16
		err := binary.Read(reader, binary.LittleEndian, &value)
		return value, true, err
	case ggufTypeInt16:
		var value int16
		err := binary.Read(reader, binary.LittleEndian, &value)
		return value, true, err
	case ggufTypeUint32:
		var value uint32
		err := binary.Read(reader, binary.LittleEndian, &value)
		return value, true, err
	case ggufTypeInt32:
		var value int32
		err := binary.Read(reader, binary.LittleEndian, &value)
		return value, true, err
	case ggufTypeFloat32:
		var value float32
		err := binary.Read(reader, binary.LittleEndian, &value)
		return value, true, err
	case ggufTypeBool:
		var value uint8
		err := binary.Read(reader, binary.LittleEndian, &value)
		return value != 0, true, err
	case ggufTypeString:
		value, err := readGGUFString(reader)
		return value, true, err
	case ggufTypeArray:
		if err := skipGGUFArray(reader); err != nil {
			return nil, false, err
		}
		return nil, false, nil
	case ggufTypeUint64:
		var value uint64
		err := binary.Read(reader, binary.LittleEndian, &value)
		return value, true, err
	case ggufTypeInt64:
		var value int64
		err := binary.Read(reader, binary.LittleEndian, &value)
		return value, true, err
	case ggufTypeFloat64:
		var value float64
		err := binary.Read(reader, binary.LittleEndian, &value)
		return value, true, err
	default:
		return nil, false, fmt.Errorf("unknown GGUF value type %d", valueType)
	}
}

func skipGGUFArray(reader io.ReadSeeker) error {
	elementType, err := readUint32(reader)
	if err != nil {
		return err
	}
	length, err := readUint64(reader)
	if err != nil {
		return err
	}

	if elementType == ggufTypeString {
		for i := uint64(0); i < length; i++ {
			if err := skipGGUFString(reader); err != nil {
				return err
			}
		}
		return nil
	}
	if elementType == ggufTypeArray {
		for i := uint64(0); i < length; i++ {
			if err := skipGGUFArray(reader); err != nil {
				return err
			}
		}
		return nil
	}

	size, ok := ggufScalarTypeSize(elementType)
	if !ok {
		return fmt.Errorf("unknown GGUF array element type %d", elementType)
	}
	if length > math.MaxInt64/uint64(size) {
		return fmt.Errorf("GGUF array is too large to skip")
	}
	_, err = reader.Seek(int64(length*uint64(size)), io.SeekCurrent)
	return err
}

func ggufScalarTypeSize(valueType uint32) (int, bool) {
	switch valueType {
	case ggufTypeUint8, ggufTypeInt8, ggufTypeBool:
		return 1, true
	case ggufTypeUint16, ggufTypeInt16:
		return 2, true
	case ggufTypeUint32, ggufTypeInt32, ggufTypeFloat32:
		return 4, true
	case ggufTypeUint64, ggufTypeInt64, ggufTypeFloat64:
		return 8, true
	default:
		return 0, false
	}
}

func readGGUFString(reader io.Reader) (string, error) {
	length, err := readUint64(reader)
	if err != nil {
		return "", err
	}
	if length > maxGGUFStringLength {
		return "", fmt.Errorf("GGUF string is too large")
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func skipGGUFString(reader io.ReadSeeker) error {
	length, err := readUint64(reader)
	if err != nil {
		return err
	}
	if length > math.MaxInt64 {
		return fmt.Errorf("GGUF string is too large to skip")
	}
	_, err = reader.Seek(int64(length), io.SeekCurrent)
	return err
}

func readUint32(reader io.Reader) (uint32, error) {
	var value uint32
	err := binary.Read(reader, binary.LittleEndian, &value)
	return value, err
}

func readUint64(reader io.Reader) (uint64, error) {
	var value uint64
	err := binary.Read(reader, binary.LittleEndian, &value)
	return value, err
}

func ggufInfoFromMetadata(modelPath string, version uint32, metadata map[string]any) ggufModelInfo {
	architecture := stringMetadata(metadata, "general.architecture")
	prefix := architecture
	if prefix == "" {
		prefix = "llama"
	}
	fileType := intMetadata(metadata, "general.file_type")
	contextLength := intMetadata(metadata, prefix+".context_length")
	blockCount := intMetadata(metadata, prefix+".block_count")
	batchMax := 8192
	if contextLength > 0 && contextLength < batchMax {
		batchMax = contextLength
	}
	if batchMax < 512 {
		batchMax = 512
	}

	return ggufModelInfo{
		Path:              modelPath,
		Exists:            true,
		Format:            "gguf",
		Version:           version,
		Name:              stringMetadata(metadata, "general.name"),
		Architecture:      architecture,
		Quantization:      ggufFileTypeName(fileType),
		FileType:          fileType,
		ContextLength:     contextLength,
		BlockCount:        blockCount,
		EmbeddingLength:   intMetadata(metadata, prefix+".embedding_length"),
		FeedForwardLength: intMetadata(metadata, prefix+".feed_forward_length"),
		HeadCount:         intMetadata(metadata, prefix+".attention.head_count"),
		HeadCountKV:       intMetadata(metadata, prefix+".attention.head_count_kv"),
		VocabularySize:    intMetadata(metadata, prefix+".vocab_size"),
		Limits: ggufModelLimits{
			ContextMax:    contextLength,
			GPULayerMax:   max(0, blockCount+1),
			BatchMax:      batchMax,
			MicroBatchMax: batchMax,
			ThreadsMax:    max(1, runtime.NumCPU()),
			ParallelMax:   16,
		},
	}
}

func validateGGUFRuntimeLimits(cmd string) error {
	args, ok := llamaServerCommandArgs(cmd)
	if !ok {
		return nil
	}
	modelPath, ok := flagValue(args, "--model", "-m")
	if !ok || modelPath == "" || strings.Contains(modelPath, "${") {
		return nil
	}

	info, err := inspectGGUFModel(modelPath)
	if err != nil {
		return fmt.Errorf("model metadata check failed: %w", err)
	}

	if info.ContextLength > 0 {
		if value, ok := intFlagValue(args, "--ctx-size", "-c"); ok && value > info.ContextLength {
			return fmt.Errorf("ctx-size %d exceeds model context length %d", value, info.ContextLength)
		}
		if value, ok := intFlagValue(args, "--batch-size", "-b"); ok && value > info.ContextLength {
			return fmt.Errorf("batch-size %d exceeds model context length %d", value, info.ContextLength)
		}
		if value, ok := intFlagValue(args, "--ubatch-size", "-ub"); ok && value > info.ContextLength {
			return fmt.Errorf("ubatch-size %d exceeds model context length %d", value, info.ContextLength)
		}
	}

	if info.BlockCount > 0 {
		maxLayers := info.BlockCount + 1
		if value, ok := intFlagValue(args, "--gpu-layers", "--n-gpu-layers", "-ngl"); ok && value > maxLayers {
			return fmt.Errorf("gpu-layers %d exceeds model layer limit %d", value, maxLayers)
		}
	}

	return nil
}

func modelPathFromCommand(cmd string) (string, bool) {
	args, ok := llamaServerCommandArgs(cmd)
	if !ok {
		return "", false
	}
	return flagValue(args, "--model", "-m")
}

func llamaServerCommandArgs(cmd string) ([]string, bool) {
	args, err := config.SanitizeCommand(cmd)
	if err != nil || len(args) == 0 {
		return nil, false
	}
	return args, strings.Contains(filepath.Base(args[0]), "llama-server")
}

func flagValue(args []string, names ...string) (string, bool) {
	for i := 0; i < len(args); i++ {
		for _, name := range names {
			if args[i] == name && i+1 < len(args) {
				return args[i+1], true
			}
			prefix := name + "="
			if strings.HasPrefix(args[i], prefix) {
				return strings.TrimPrefix(args[i], prefix), true
			}
		}
	}
	return "", false
}

func intFlagValue(args []string, names ...string) (int, bool) {
	raw, ok := flagValue(args, names...)
	if !ok || raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func stringMetadata(metadata map[string]any, key string) string {
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}

func intMetadata(metadata map[string]any, key string) int {
	value, ok := metadata[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case uint8:
		return int(typed)
	case int8:
		return int(typed)
	case uint16:
		return int(typed)
	case int16:
		return int(typed)
	case uint32:
		if typed > uint32(maxIntValue()) {
			return 0
		}
		return int(typed)
	case int32:
		return int(typed)
	case uint64:
		if typed > uint64(maxIntValue()) {
			return 0
		}
		return int(typed)
	case int64:
		if typed > int64(maxIntValue()) || typed < int64(minIntValue()) {
			return 0
		}
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func maxIntValue() int {
	return int(^uint(0) >> 1)
}

func minIntValue() int {
	return -maxIntValue() - 1
}

func ggufFileTypeName(fileType int) string {
	names := map[int]string{
		0:  "ALL_F32",
		1:  "MOSTLY_F16",
		2:  "MOSTLY_Q4_0",
		3:  "MOSTLY_Q4_1",
		6:  "MOSTLY_Q5_0",
		7:  "MOSTLY_Q5_1",
		8:  "MOSTLY_Q8_0",
		10: "MOSTLY_Q2_K",
		11: "MOSTLY_Q3_K_S",
		12: "MOSTLY_Q3_K_M",
		13: "MOSTLY_Q3_K_L",
		14: "MOSTLY_Q4_K_S",
		15: "MOSTLY_Q4_K_M",
		16: "MOSTLY_Q5_K_S",
		17: "MOSTLY_Q5_K_M",
		18: "MOSTLY_Q6_K",
		19: "MOSTLY_IQ2_XXS",
		20: "MOSTLY_IQ2_XS",
		21: "MOSTLY_Q2_K_S",
		22: "MOSTLY_IQ3_XS",
		23: "MOSTLY_IQ3_XXS",
		24: "MOSTLY_IQ1_S",
		25: "MOSTLY_IQ4_NL",
		26: "MOSTLY_IQ3_S",
		27: "MOSTLY_IQ3_M",
		28: "MOSTLY_IQ2_S",
		29: "MOSTLY_IQ2_M",
		30: "MOSTLY_IQ4_XS",
		31: "MOSTLY_IQ1_M",
		32: "MOSTLY_BF16",
	}
	if name, ok := names[fileType]; ok {
		return name
	}
	if fileType == 0 {
		return ""
	}
	return fmt.Sprintf("type %d", fileType)
}
