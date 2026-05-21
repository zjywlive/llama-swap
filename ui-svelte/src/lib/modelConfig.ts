import type { EditableModelInfo } from "./types";

export interface LlamaServerRuntime {
  executable: string;
  modelPath: string;
  host: string;
  port: string;
  ctxSize: number | "";
  threads: number | "";
  threadsBatch: number | "";
  batchSize: number | "";
  ubatchSize: number | "";
  parallel: number | "";
  priority: number | "";
  device: string;
  gpuLayers: number | "";
  flashAttention: "" | "auto" | "on" | "off";
  noWarmup: boolean;
  cacheTypeK: string;
  cacheTypeV: string;
  extraArgs: string;
}

export interface MLXServerRuntime {
  executable: string;
  modelPath: string;
  host: string;
  port: string;
  adapterPath: string;
  allowedOrigins: string;
  draftModel: string;
  numDraftTokens: number | "";
  trustRemoteCode: boolean;
  logLevel: "" | "DEBUG" | "INFO" | "WARNING" | "ERROR" | "CRITICAL";
  chatTemplate: string;
  useDefaultChatTemplate: boolean;
  temp: number | "";
  topP: number | "";
  topK: number | "";
  minP: number | "";
  maxTokens: number | "";
  chatTemplateArgs: string;
  decodeConcurrency: number | "";
  promptConcurrency: number | "";
  prefillStepSize: number | "";
  promptCacheSize: number | "";
  promptCacheBytes: string;
  pipeline: boolean;
  extraArgs: string;
}

export interface ParsedRuntime {
  kind: "llama-server" | "mlx-lm" | "raw";
  runtime: LlamaServerRuntime;
  mlxRuntime: MLXServerRuntime;
}

export interface RuntimeMemoryEstimate {
  totalBytes: number;
  gpuBytes: number;
  modelBytes: number;
  kvBytes: number;
  overheadBytes: number;
  confidence: "medium" | "low";
  notes: string[];
}

export function defaultRuntime(): LlamaServerRuntime {
  return {
    executable: "llama-server",
    modelPath: "",
    host: "127.0.0.1",
    port: "${PORT}",
    ctxSize: "",
    threads: "",
    threadsBatch: "",
    batchSize: "",
    ubatchSize: "",
    parallel: "",
    priority: "",
    device: "",
    gpuLayers: "",
    flashAttention: "",
    noWarmup: false,
    cacheTypeK: "",
    cacheTypeV: "",
    extraArgs: "",
  };
}

export function defaultMLXRuntime(): MLXServerRuntime {
  return {
    executable: "/Users/rick/.local/bin/mlx_lm.server",
    modelPath: "",
    host: "127.0.0.1",
    port: "${PORT}",
    adapterPath: "",
    allowedOrigins: "",
    draftModel: "",
    numDraftTokens: "",
    trustRemoteCode: false,
    logLevel: "",
    chatTemplate: "",
    useDefaultChatTemplate: false,
    temp: 0,
    topP: 1,
    topK: "",
    minP: "",
    maxTokens: 512,
    chatTemplateArgs: "",
    decodeConcurrency: "",
    promptConcurrency: "",
    prefillStepSize: 2048,
    promptCacheSize: "",
    promptCacheBytes: "",
    pipeline: false,
    extraArgs: "",
  };
}

export function shellSplit(input: string): string[] {
  const out: string[] = [];
  let current = "";
  let quote: "'" | '"' | null = null;
  let escaping = false;

  for (const char of input) {
    if (escaping) {
      current += char;
      escaping = false;
      continue;
    }
    if (char === "\\" && quote !== "'") {
      escaping = true;
      continue;
    }
    if ((char === "'" || char === '"') && quote === null) {
      quote = char;
      continue;
    }
    if (char === quote) {
      quote = null;
      continue;
    }
    if (/\s/.test(char) && quote === null) {
      if (current !== "") {
        out.push(current);
        current = "";
      }
      continue;
    }
    current += char;
  }

  if (current !== "") out.push(current);
  return out;
}

function basename(path: string): string {
  return path.split(/[\\/]/).pop() || path;
}

function asNumber(value: string | undefined): number | "" {
  if (value === undefined || value.trim() === "") return "";
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : "";
}

function quoteArg(arg: string): string {
  if (arg === "") return "''";
  if (/^[A-Za-z0-9_./:=,+@%{}$-]+$/.test(arg)) return arg;
  return `'${arg.replace(/'/g, `'\\''`)}'`;
}

function appendValue(args: string[], flag: string, value: string | number | ""): void {
  if (value === "" || value === undefined || value === null) return;
  args.push(flag, String(value));
}

function appendBoolean(args: string[], flag: string, value: boolean): void {
  if (value) args.push(flag);
}

function numericValue(value: string | number | "", fallback = 0): number {
  if (value === "") return fallback;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function boundedDefault(preferred: number, max: number | undefined, min = 1): number {
  const limit = max && max > 0 ? max : preferred;
  return Math.max(min, Math.min(preferred, limit));
}

function modelLayerCount(info: EditableModelInfo | null | undefined): number {
  if (!info) return 0;
  if (info.blockCount > 0) return info.blockCount;
  if (info.limits?.gpuLayerMax > 0) return info.limits.gpuLayerMax;
  return 0;
}

export function createLlamaStudioRuntimeDefaults(modelPath: string, info: EditableModelInfo | null | undefined, alias = ""): LlamaServerRuntime {
  const runtime = defaultRuntime();
  const contextMax = info?.limits?.contextMax || info?.contextLength || 4096;
  const ctxSize = boundedDefault(contextMax, contextMax, 512);
  const threads = boundedDefault(16, info?.limits?.threadsMax, 1);
  const batchMax = Math.min(info?.limits?.batchMax || ctxSize, ctxSize);
  const microBatchMax = Math.min(info?.limits?.microBatchMax || batchMax, batchMax);

  runtime.modelPath = modelPath || info?.path || "";
  runtime.ctxSize = ctxSize;
  runtime.threads = threads;
  runtime.threadsBatch = threads;
  runtime.batchSize = boundedDefault(1024, batchMax, 32);
  runtime.ubatchSize = boundedDefault(512, microBatchMax, 32);
  runtime.parallel = 1;
  runtime.priority = -1;
  runtime.gpuLayers = modelLayerCount(info);
  runtime.flashAttention = "on";
  runtime.noWarmup = true;
  runtime.extraArgs = alias ? `--alias ${alias}` : "";
  return runtime;
}

function cacheElementBytes(cacheType: string): number {
  const normalized = cacheType.trim().toLowerCase();
  if (!normalized || normalized === "auto" || normalized === "f16" || normalized === "bf16") return 2;
  if (normalized === "f32") return 4;
  if (normalized === "q8_0") return 1;
  if (normalized.startsWith("q6")) return 0.75;
  if (normalized.startsWith("q5")) return 0.625;
  if (normalized.startsWith("q4") || normalized.startsWith("iq4")) return 0.5;
  return 2;
}

function kvCacheBytes(info: EditableModelInfo, runtime: LlamaServerRuntime): number {
  const context = numericValue(runtime.ctxSize, info.contextLength || 0);
  const parallel = Math.max(1, numericValue(runtime.parallel, 1));
  const layers = modelLayerCount(info);
  const embedding = info.embeddingLength;
  const heads = info.headCount;
  const kvHeads = info.headCountKV || heads;

  if (context <= 0 || layers <= 0 || embedding <= 0 || heads <= 0 || kvHeads <= 0) return 0;

  const headSize = embedding / heads;
  const kvWidth = headSize * kvHeads;
  const kBytes = cacheElementBytes(runtime.cacheTypeK);
  const vBytes = cacheElementBytes(runtime.cacheTypeV);
  return context * parallel * layers * kvWidth * (kBytes + vBytes);
}

export function estimateLlamaMemoryUsage(info: EditableModelInfo | null | undefined, runtime: LlamaServerRuntime): RuntimeMemoryEstimate | null {
  if (!info) return null;
  const notes: string[] = [];
  const modelBytes = Math.max(0, info.fileSize || 0);
  const layers = modelLayerCount(info);
  const gpuLayers = Math.max(0, numericValue(runtime.gpuLayers, 0));
  const layerFraction = layers > 0 ? Math.min(gpuLayers, layers) / layers : 0;
  const kvBytes = kvCacheBytes(info, runtime);

  if (modelBytes === 0) notes.push("model-size-missing");
  if (kvBytes === 0) notes.push("kv-metadata-missing");

  const overheadBytes = modelBytes > 0 || kvBytes > 0 ? Math.max(256 * 1024 * 1024, (modelBytes + kvBytes) * 0.03) : 0;
  const modelGpuBytes = modelBytes * layerFraction;
  const kvGpuBytes = gpuLayers > 0 ? kvBytes : 0;
  const gpuOverheadBytes = gpuLayers > 0 ? overheadBytes : 0;

  return {
    totalBytes: modelBytes + kvBytes + overheadBytes,
    gpuBytes: modelGpuBytes + kvGpuBytes + gpuOverheadBytes,
    modelBytes,
    kvBytes,
    overheadBytes,
    confidence: notes.length > 0 ? "low" : "medium",
    notes,
  };
}

export function estimateMLXMemoryUsage(info: EditableModelInfo | null | undefined): RuntimeMemoryEstimate | null {
  if (!info) return null;
  const modelBytes = Math.max(0, info.fileSize || 0);
  const overheadBytes = modelBytes > 0 ? Math.max(256 * 1024 * 1024, modelBytes * 0.08) : 0;
  return {
    totalBytes: modelBytes + overheadBytes,
    gpuBytes: modelBytes + overheadBytes,
    modelBytes,
    kvBytes: 0,
    overheadBytes,
    confidence: "low",
    notes: modelBytes > 0 ? ["mlx-runtime-estimate"] : ["model-size-missing"],
  };
}

export function formatMemorySize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "unknown";
  const gb = bytes / (1024 ** 3);
  if (gb >= 1) return `${gb.toFixed(gb >= 10 ? 1 : 2)} GB`;
  const mb = bytes / (1024 ** 2);
  return `${mb.toFixed(0)} MB`;
}

export function parseRuntimeCommand(cmd: string): ParsedRuntime {
  const runtime = defaultRuntime();
  const mlxRuntime = defaultMLXRuntime();
  const tokens = shellSplit(cmd);
  if (tokens.length === 0) {
    runtime.extraArgs = cmd;
    return { kind: "raw", runtime, mlxRuntime };
  }

  const executableName = basename(tokens[0]);
  if (executableName === "mlx_lm.server" || executableName.includes("mlx-lm")) {
    mlxRuntime.executable = tokens[0];
    const extra: string[] = [];
    const takeValue = (index: number) => (index + 1 < tokens.length ? tokens[index + 1] : undefined);

    for (let i = 1; i < tokens.length; i++) {
      const flag = tokens[i];
      const value = takeValue(i);
      const consume = () => { i++; };

      switch (flag) {
        case "--model":
          mlxRuntime.modelPath = value ?? "";
          consume();
          break;
        case "--host":
          mlxRuntime.host = value ?? "";
          consume();
          break;
        case "--port":
          mlxRuntime.port = value ?? "";
          consume();
          break;
        case "--adapter-path":
          mlxRuntime.adapterPath = value ?? "";
          consume();
          break;
        case "--allowed-origins":
          mlxRuntime.allowedOrigins = value ?? "";
          consume();
          break;
        case "--draft-model":
          mlxRuntime.draftModel = value ?? "";
          consume();
          break;
        case "--num-draft-tokens":
          mlxRuntime.numDraftTokens = asNumber(value);
          consume();
          break;
        case "--trust-remote-code":
          mlxRuntime.trustRemoteCode = true;
          break;
        case "--log-level":
          mlxRuntime.logLevel = (["DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"].includes(value ?? "") ? value : "") as MLXServerRuntime["logLevel"];
          consume();
          break;
        case "--chat-template":
          mlxRuntime.chatTemplate = value ?? "";
          consume();
          break;
        case "--use-default-chat-template":
          mlxRuntime.useDefaultChatTemplate = true;
          break;
        case "--temp":
          mlxRuntime.temp = asNumber(value);
          consume();
          break;
        case "--top-p":
          mlxRuntime.topP = asNumber(value);
          consume();
          break;
        case "--top-k":
          mlxRuntime.topK = asNumber(value);
          consume();
          break;
        case "--min-p":
          mlxRuntime.minP = asNumber(value);
          consume();
          break;
        case "--max-tokens":
          mlxRuntime.maxTokens = asNumber(value);
          consume();
          break;
        case "--chat-template-args":
          mlxRuntime.chatTemplateArgs = value ?? "";
          consume();
          break;
        case "--decode-concurrency":
          mlxRuntime.decodeConcurrency = asNumber(value);
          consume();
          break;
        case "--prompt-concurrency":
          mlxRuntime.promptConcurrency = asNumber(value);
          consume();
          break;
        case "--prefill-step-size":
          mlxRuntime.prefillStepSize = asNumber(value);
          consume();
          break;
        case "--prompt-cache-size":
          mlxRuntime.promptCacheSize = asNumber(value);
          consume();
          break;
        case "--prompt-cache-bytes":
          mlxRuntime.promptCacheBytes = value ?? "";
          consume();
          break;
        case "--pipeline":
          mlxRuntime.pipeline = true;
          break;
        default:
          extra.push(flag);
          if (value && !value.startsWith("-")) {
            extra.push(value);
            consume();
          }
          break;
      }
    }

    mlxRuntime.extraArgs = extra.map(quoteArg).join(" ");
    return { kind: "mlx-lm", runtime, mlxRuntime };
  }

  if (!executableName.includes("llama-server")) {
    runtime.extraArgs = cmd;
    return { kind: "raw", runtime, mlxRuntime };
  }

  runtime.executable = tokens[0];
  const extra: string[] = [];
  const takeValue = (index: number) => (index + 1 < tokens.length ? tokens[index + 1] : undefined);

  for (let i = 1; i < tokens.length; i++) {
    const flag = tokens[i];
    const value = takeValue(i);
    const consume = () => { i++; };

    switch (flag) {
      case "--model":
      case "-m":
        runtime.modelPath = value ?? "";
        consume();
        break;
      case "--host":
        runtime.host = value ?? "";
        consume();
        break;
      case "--port":
        runtime.port = value ?? "";
        consume();
        break;
      case "--ctx-size":
      case "-c":
        runtime.ctxSize = asNumber(value);
        consume();
        break;
      case "--threads":
      case "-t":
        runtime.threads = asNumber(value);
        consume();
        break;
      case "--threads-batch":
      case "-tb":
        runtime.threadsBatch = asNumber(value);
        consume();
        break;
      case "--batch-size":
      case "-b":
        runtime.batchSize = asNumber(value);
        consume();
        break;
      case "--ubatch-size":
      case "-ub":
        runtime.ubatchSize = asNumber(value);
        consume();
        break;
      case "--parallel":
      case "-np":
        runtime.parallel = asNumber(value);
        consume();
        break;
      case "--prio":
        runtime.priority = asNumber(value);
        consume();
        break;
      case "--device":
      case "-dev":
        runtime.device = value ?? "";
        consume();
        break;
      case "--gpu-layers":
      case "--n-gpu-layers":
      case "-ngl":
        {
          const parsed = asNumber(value);
          if (parsed === "" && value) {
            extra.push(flag, value);
          } else {
            runtime.gpuLayers = parsed;
          }
        }
        consume();
        break;
      case "--flash-attn":
      case "-fa":
        runtime.flashAttention = (value === "auto" || value === "on" || value === "off") ? value : "";
        consume();
        break;
      case "--no-warmup":
        runtime.noWarmup = true;
        break;
      case "--warmup":
        runtime.noWarmup = false;
        break;
      case "--cache-type-k":
      case "-ctk":
        runtime.cacheTypeK = value ?? "";
        consume();
        break;
      case "--cache-type-v":
      case "-ctv":
        runtime.cacheTypeV = value ?? "";
        consume();
        break;
      default:
        extra.push(flag);
        if (value && !value.startsWith("-")) {
          extra.push(value);
          consume();
        }
        break;
    }
  }

  runtime.extraArgs = extra.map(quoteArg).join(" ");
  return { kind: "llama-server", runtime, mlxRuntime };
}

export function buildLlamaServerCommand(runtime: LlamaServerRuntime): string {
  const args: string[] = [runtime.executable || "llama-server"];
  appendValue(args, "--host", runtime.host);
  appendValue(args, "--port", runtime.port);
  appendValue(args, "--model", runtime.modelPath);
  appendValue(args, "--ctx-size", runtime.ctxSize);
  appendValue(args, "--threads", runtime.threads);
  appendValue(args, "--threads-batch", runtime.threadsBatch);
  appendValue(args, "--batch-size", runtime.batchSize);
  appendValue(args, "--ubatch-size", runtime.ubatchSize);
  appendValue(args, "--parallel", runtime.parallel);
  appendValue(args, "--prio", runtime.priority);
  appendValue(args, "--device", runtime.device);
  appendValue(args, "--gpu-layers", runtime.gpuLayers);
  appendValue(args, "--flash-attn", runtime.flashAttention);
  appendValue(args, "--cache-type-k", runtime.cacheTypeK);
  appendValue(args, "--cache-type-v", runtime.cacheTypeV);
  if (runtime.noWarmup) args.push("--no-warmup");

  const command = args.map(quoteArg).join(" ");
  return runtime.extraArgs.trim() ? `${command} ${runtime.extraArgs.trim()}` : command;
}

export function buildMLXServerCommand(runtime: MLXServerRuntime): string {
  const args: string[] = [runtime.executable || "/Users/rick/.local/bin/mlx_lm.server"];
  appendValue(args, "--host", runtime.host);
  appendValue(args, "--port", runtime.port);
  appendValue(args, "--model", runtime.modelPath);
  appendValue(args, "--adapter-path", runtime.adapterPath);
  appendValue(args, "--allowed-origins", runtime.allowedOrigins);
  appendValue(args, "--draft-model", runtime.draftModel);
  appendValue(args, "--num-draft-tokens", runtime.numDraftTokens);
  appendBoolean(args, "--trust-remote-code", runtime.trustRemoteCode);
  appendValue(args, "--log-level", runtime.logLevel);
  appendValue(args, "--chat-template", runtime.chatTemplate);
  appendBoolean(args, "--use-default-chat-template", runtime.useDefaultChatTemplate);
  appendValue(args, "--temp", runtime.temp);
  appendValue(args, "--top-p", runtime.topP);
  appendValue(args, "--top-k", runtime.topK);
  appendValue(args, "--min-p", runtime.minP);
  appendValue(args, "--max-tokens", runtime.maxTokens);
  appendValue(args, "--chat-template-args", runtime.chatTemplateArgs);
  appendValue(args, "--decode-concurrency", runtime.decodeConcurrency);
  appendValue(args, "--prompt-concurrency", runtime.promptConcurrency);
  appendValue(args, "--prefill-step-size", runtime.prefillStepSize);
  appendValue(args, "--prompt-cache-size", runtime.promptCacheSize);
  appendValue(args, "--prompt-cache-bytes", runtime.promptCacheBytes);
  appendBoolean(args, "--pipeline", runtime.pipeline);

  const command = args.map(quoteArg).join(" ");
  return runtime.extraArgs.trim() ? `${command} ${runtime.extraArgs.trim()}` : command;
}
