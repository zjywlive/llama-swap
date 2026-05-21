import { describe, expect, it } from "vitest";
import type { EditableModelInfo } from "./types";
import {
  createLlamaStudioRuntimeDefaults,
  estimateLlamaMemoryUsage,
  formatMemorySize,
} from "./modelConfig";

function modelInfo(): EditableModelInfo {
  return {
    path: "/models/example.gguf",
    exists: true,
    backend: "llama-server",
    format: "gguf",
    fileSize: 8_727_635_648,
    version: 3,
    name: "Example",
    architecture: "llama",
    modelType: "llama",
    quantization: "Q5_K_M",
    torchDtype: "",
    fileType: 17,
    contextLength: 131_072,
    blockCount: 40,
    embeddingLength: 5120,
    feedForwardLength: 0,
    headCount: 40,
    headCountKV: 8,
    vocabularySize: 128_256,
    limits: {
      contextMax: 131_072,
      gpuLayerMax: 41,
      batchMax: 8192,
      microBatchMax: 8192,
      threadsMax: 18,
      parallelMax: 16,
    },
    warnings: [],
  };
}

describe("modelConfig", () => {
  it("creates LM Studio style llama-server defaults", () => {
    const runtime = createLlamaStudioRuntimeDefaults("/models/example.gguf", modelInfo(), "example");

    expect(runtime.ctxSize).toBe(131_072);
    expect(runtime.threads).toBe(16);
    expect(runtime.threadsBatch).toBe(16);
    expect(runtime.batchSize).toBe(1024);
    expect(runtime.ubatchSize).toBe(512);
    expect(runtime.parallel).toBe(1);
    expect(runtime.gpuLayers).toBe(40);
    expect(runtime.flashAttention).toBe("on");
    expect(runtime.extraArgs).toBe("--alias example");
  });

  it("estimates llama-server memory from model size and KV cache", () => {
    const info = modelInfo();
    const runtime = createLlamaStudioRuntimeDefaults(info.path, info);
    const estimate = estimateLlamaMemoryUsage(info, runtime);

    expect(estimate).not.toBeNull();
    expect(estimate?.modelBytes).toBe(info.fileSize);
    expect(estimate?.kvBytes).toBeGreaterThan(0);
    expect(estimate?.totalBytes).toBeGreaterThan(info.fileSize);
    expect(estimate?.gpuBytes).toBeGreaterThan(info.fileSize);
    expect(formatMemorySize(estimate?.totalBytes ?? 0)).toContain("GB");
  });
});
