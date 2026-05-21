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
  gpuLayers: string;
  flashAttention: "" | "auto" | "on" | "off";
  noWarmup: boolean;
  cacheTypeK: string;
  cacheTypeV: string;
  extraArgs: string;
}

export interface ParsedRuntime {
  kind: "llama-server" | "raw";
  runtime: LlamaServerRuntime;
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
  if (/^[A-Za-z0-9_./:=,+@%{}-]+$/.test(arg)) return arg;
  return `'${arg.replace(/'/g, `'\\''`)}'`;
}

function appendValue(args: string[], flag: string, value: string | number | ""): void {
  if (value === "" || value === undefined || value === null) return;
  args.push(flag, String(value));
}

export function parseRuntimeCommand(cmd: string): ParsedRuntime {
  const runtime = defaultRuntime();
  const tokens = shellSplit(cmd);
  if (tokens.length === 0 || !basename(tokens[0]).includes("llama-server")) {
    runtime.extraArgs = cmd;
    return { kind: "raw", runtime };
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
        runtime.gpuLayers = value ?? "";
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
  return { kind: "llama-server", runtime };
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
