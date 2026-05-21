package proxy

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadGGUFMetadata(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("GGUF")
	require.NoError(t, binary.Write(&buf, binary.LittleEndian, uint32(3)))
	require.NoError(t, binary.Write(&buf, binary.LittleEndian, uint64(0)))
	require.NoError(t, binary.Write(&buf, binary.LittleEndian, uint64(5)))

	writeKVString(t, &buf, "general.architecture", "llama")
	writeKVString(t, &buf, "general.name", "test model")
	writeKVUint32(t, &buf, "general.file_type", 15)
	writeKVUint32(t, &buf, "llama.context_length", 4096)
	writeKVUint32(t, &buf, "llama.block_count", 32)

	metadata, version, err := readGGUFMetadata(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	require.Equal(t, uint32(3), version)

	info := ggufInfoFromMetadata("/tmp/test.gguf", version, metadata)
	require.Equal(t, "llama", info.Architecture)
	require.Equal(t, "test model", info.Name)
	require.Equal(t, "MOSTLY_Q4_K_M", info.Quantization)
	require.Equal(t, 4096, info.ContextLength)
	require.Equal(t, 32, info.BlockCount)
	require.Equal(t, 4096, info.Limits.ContextMax)
	require.Equal(t, 33, info.Limits.GPULayerMax)
}

func writeKVString(t *testing.T, buf *bytes.Buffer, key string, value string) {
	t.Helper()
	writeGGUFString(t, buf, key)
	require.NoError(t, binary.Write(buf, binary.LittleEndian, ggufTypeString))
	writeGGUFString(t, buf, value)
}

func writeKVUint32(t *testing.T, buf *bytes.Buffer, key string, value uint32) {
	t.Helper()
	writeGGUFString(t, buf, key)
	require.NoError(t, binary.Write(buf, binary.LittleEndian, ggufTypeUint32))
	require.NoError(t, binary.Write(buf, binary.LittleEndian, value))
}

func writeGGUFString(t *testing.T, buf *bytes.Buffer, value string) {
	t.Helper()
	require.NoError(t, binary.Write(buf, binary.LittleEndian, uint64(len(value))))
	buf.WriteString(value)
}
