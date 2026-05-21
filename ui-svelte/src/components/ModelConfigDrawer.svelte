<script lang="ts">
  import { fetchEditableModel, inspectEditableModelPath, saveEditableModel, validateEditableModel } from "../stores/api";
  import { tx } from "../stores/i18n";
  import type { EditableModelConfig, EditableModelInfo } from "../lib/types";
  import {
    buildLlamaServerCommand,
    buildMLXServerCommand,
    defaultMLXRuntime,
    defaultRuntime,
    parseRuntimeCommand,
    type LlamaServerRuntime,
    type MLXServerRuntime,
  } from "../lib/modelConfig";
  import SliderNumber from "./SliderNumber.svelte";

  interface Props {
    modelId: string | null;
    onClose: () => void;
  }

  let { modelId, onClose }: Props = $props();

  type Tab = "basic" | "runtime" | "requests" | "advanced";

  let activeTab = $state<Tab>("basic");
  let loadedModelId = $state<string | null>(null);
  let config = $state<EditableModelConfig | null>(null);
  let runtimeKind = $state<"llama-server" | "mlx-lm" | "raw">("raw");
  let runtime = $state<LlamaServerRuntime>(defaultRuntime());
  let mlxRuntime = $state<MLXServerRuntime>(defaultMLXRuntime());
  let modelInfo = $state<EditableModelInfo | null>(null);
  let rawCommand = $state("");
  let aliasesText = $state("");
  let envText = $state("");
  let setParamsText = $state("{}");
  let setParamsByIDText = $state("{}");
  let isLoading = $state(false);
  let isSaving = $state(false);
  let isValidating = $state(false);
  let error = $state<string | null>(null);
  let message = $state<string | null>(null);

  let commandPreview = $derived(runtimeKind === "llama-server" ? buildLlamaServerCommand(runtime) : runtimeKind === "mlx-lm" ? buildMLXServerCommand(mlxRuntime) : rawCommand);
  let contextMax = $derived(modelInfo?.limits?.contextMax || 131072);
  let gpuLayerMax = $derived(modelInfo?.limits?.gpuLayerMax || 128);
  let batchMax = $derived(modelInfo?.limits?.batchMax || Math.min(contextMax, 8192));
  let microBatchMax = $derived(modelInfo?.limits?.microBatchMax || batchMax);
  let threadsMax = $derived(modelInfo?.limits?.threadsMax || (typeof navigator !== "undefined" ? navigator.hardwareConcurrency || 8 : 8));
  let parallelMax = $derived(modelInfo?.limits?.parallelMax || 16);

  $effect(() => {
    if (modelId && modelId !== loadedModelId) {
      void loadConfig(modelId);
    }
  });

  function cloneConfig(value: EditableModelConfig): EditableModelConfig {
    return JSON.parse(JSON.stringify(value)) as EditableModelConfig;
  }

  async function loadConfig(id: string): Promise<void> {
    isLoading = true;
    error = null;
    message = null;
    loadedModelId = id;
    activeTab = "basic";
    try {
      const loaded = await fetchEditableModel(id);
      config = cloneConfig(loaded);
      aliasesText = loaded.aliases?.join(", ") ?? "";
      envText = loaded.env?.join("\n") ?? "";
      setParamsText = JSON.stringify(loaded.filters?.setParams ?? {}, null, 2);
      setParamsByIDText = JSON.stringify(loaded.filters?.setParamsByID ?? {}, null, 2);
      const parsed = parseRuntimeCommand(loaded.cmd);
      runtimeKind = parsed.kind;
      runtime = parsed.runtime;
      mlxRuntime = parsed.mlxRuntime;
      rawCommand = loaded.cmd;
      modelInfo = loaded.modelInfo ?? null;
      if (parsed.kind === "llama-server" && parsed.runtime.modelPath && !loaded.modelInfo) {
        void inspectModelPath(parsed.runtime.modelPath);
      } else if (parsed.kind === "mlx-lm" && parsed.mlxRuntime.modelPath && !loaded.modelInfo) {
        void inspectModelPath(parsed.mlxRuntime.modelPath);
      }
    } catch (err) {
      error = err instanceof Error ? err.message : $tx.editor.loadingFailed;
    } finally {
      isLoading = false;
    }
  }

  function splitComma(text: string): string[] {
    return text.split(",").map((item) => item.trim()).filter(Boolean);
  }

  function splitLines(text: string): string[] {
    return text.split(/\r?\n/).map((item) => item.trim()).filter(Boolean);
  }

  function parseObject(text: string, label: string): Record<string, unknown> {
    const trimmed = text.trim();
    if (!trimmed) return {};
    const value = JSON.parse(trimmed);
    if (value === null || typeof value !== "object" || Array.isArray(value)) {
      throw new Error(`${label} must be a JSON object`);
    }
    return value as Record<string, unknown>;
  }

  function buildConfigForSubmit(): EditableModelConfig {
    if (!config) throw new Error("No model config loaded");
    const setParams = parseObject(setParamsText, "setParams");
    const setParamsByID = parseObject(setParamsByIDText, "setParamsByID") as Record<string, Record<string, unknown>>;

    return {
      ...config,
      cmd: commandPreview,
      aliases: splitComma(aliasesText),
      env: splitLines(envText),
      filters: {
        ...(config.filters ?? { stripParams: "", setParams: {}, setParamsByID: {} }),
        setParams,
        setParamsByID,
      },
    };
  }

  function handleSendLoadingStateChange(event: Event): void {
    if (!config) return;
    const value = (event.target as HTMLSelectElement).value;
    config.sendLoadingState = value === "inherit" ? null : value === "true";
  }

  async function inspectModelPath(path: string): Promise<void> {
    const trimmed = path.trim();
    if (!trimmed || trimmed.includes("${")) {
      modelInfo = null;
      return;
    }
    try {
      modelInfo = await inspectEditableModelPath(trimmed);
    } catch (err) {
      modelInfo = {
        path: trimmed,
        exists: false,
        backend: "",
        format: "",
        version: 0,
        name: "",
        architecture: "",
        modelType: "",
        quantization: "",
        torchDtype: "",
        fileType: 0,
        contextLength: 0,
        blockCount: 0,
        embeddingLength: 0,
        feedForwardLength: 0,
        headCount: 0,
        headCountKV: 0,
        vocabularySize: 0,
        limits: { contextMax, gpuLayerMax, batchMax, microBatchMax, threadsMax, parallelMax },
        warnings: [`${$tx.editor.inspectFailed}: ${err instanceof Error ? err.message : String(err)}`],
      };
    }
  }

  function numberOrUnknown(value: number): string {
    return value > 0 ? value.toLocaleString() : $tx.editor.modelInfo.unknown;
  }

  function maxHelp(label: string, max: number): string {
    return `${label}: <= ${max.toLocaleString()}. ${$tx.editor.help.runtimeLimited}`;
  }

  function runtimeHelp(): string {
    if (runtimeKind === "llama-server") return $tx.editor.help.runtimeAuto;
    if (runtimeKind === "mlx-lm") return $tx.editor.help.runtimeMLX;
    return $tx.editor.help.runtimeRaw;
  }

  async function validate(): Promise<void> {
    isValidating = true;
    error = null;
    message = null;
    try {
      await validateEditableModel(buildConfigForSubmit());
      message = $tx.editor.validationOk;
    } catch (err) {
      error = `${$tx.editor.validationFailed}: ${err instanceof Error ? err.message : String(err)}`;
    } finally {
      isValidating = false;
    }
  }

  async function save(): Promise<void> {
    isSaving = true;
    error = null;
    message = null;
    try {
      const next = buildConfigForSubmit();
      await saveEditableModel(next);
      config = cloneConfig(next);
      message = $tx.editor.help.saveReload;
    } catch (err) {
      error = `${$tx.editor.saveFailed}: ${err instanceof Error ? err.message : String(err)}`;
    } finally {
      isSaving = false;
    }
  }
</script>

{#if modelId}
  <div class="fixed inset-0 z-40 bg-black/30" role="button" tabindex="0" aria-label={$tx.common.close} onclick={onClose} onkeydown={(e) => e.key === "Escape" && onClose()}></div>
  <aside class="fixed right-0 top-0 z-50 flex h-screen w-full max-w-3xl flex-col border-l border-card-border bg-surface shadow-xl">
    <div class="shrink-0 border-b border-card-border p-4">
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0">
          <h2 class="p-0 text-2xl">{$tx.editor.title}</h2>
          <p class="mt-1 truncate text-sm text-txtsecondary">{modelId}</p>
        </div>
        <button class="btn btn--sm" onclick={onClose}>{$tx.common.close}</button>
      </div>
      <p class="mt-2 text-sm text-txtsecondary">{$tx.editor.subtitle}</p>
    </div>

    {#if isLoading}
      <div class="flex-1 p-4 text-txtsecondary">{$tx.common.loading}</div>
    {:else if config}
      <div class="flex shrink-0 gap-2 overflow-x-auto border-b border-card-border px-4 py-3">
        {#each ["basic", "runtime", "requests", "advanced"] as tab}
          <button
            class="btn btn--sm whitespace-nowrap"
            class:bg-primary={activeTab === tab}
            class:text-btn-primary-text={activeTab === tab}
            onclick={() => (activeTab = tab as Tab)}
          >
            {$tx.editor.tabs[tab as Tab]}
          </button>
        {/each}
      </div>

      <div class="flex-1 overflow-y-auto p-4">
        {#if error}
          <div class="mb-4 rounded border border-error/30 bg-error/10 p-3 text-sm text-error">{error}</div>
        {/if}
        {#if message}
          <div class="mb-4 rounded border border-success/30 bg-success/10 p-3 text-sm text-success">{message}</div>
        {/if}

        {#if activeTab === "basic"}
          <div class="grid gap-4 md:grid-cols-2">
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.id}</span>
              <input class="w-full rounded border border-border bg-card px-3 py-2" value={config.id} disabled />
            </label>
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.displayName}</span>
              <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={config.name} />
            </label>
            <label class="block md:col-span-2">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.description}</span>
              <textarea class="min-h-20 w-full rounded border border-border bg-card px-3 py-2" bind:value={config.description}></textarea>
            </label>
            <label class="block md:col-span-2">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.aliases}</span>
              <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={aliasesText} />
              <span class="mt-1 block text-xs text-txtsecondary">{$tx.editor.help.aliases}</span>
            </label>
            <SliderNumber label={$tx.editor.fields.ttl} bind:value={config.ttl} min={-1} max={86400} step={30} help={$tx.editor.help.ttl} allowEmpty={false} />
            <label class="mt-7 flex items-center gap-2">
              <input type="checkbox" bind:checked={config.unlisted} />
              <span>{$tx.editor.fields.unlisted}</span>
            </label>
          </div>
        {:else if activeTab === "runtime"}
          <div class="mb-4 rounded border border-border bg-card p-3 text-sm text-txtsecondary">
            {runtimeHelp()}
          </div>

          {#if runtimeKind !== "raw"}
            <div class="mb-4 rounded border border-border bg-card p-3 text-sm">
              <div class="mb-2 flex flex-wrap items-baseline justify-between gap-2">
                <h3 class="p-0 text-base">{$tx.editor.modelInfo.title}</h3>
                {#if modelInfo?.format}
                  <span class="text-xs uppercase text-txtsecondary">{modelInfo.backend || runtimeKind} · {modelInfo.format}{modelInfo.version ? ` v${modelInfo.version}` : ""}</span>
                {/if}
              </div>
              {#if modelInfo?.warnings?.length}
                <div class="space-y-1 text-yellow-600 dark:text-yellow-300">
                  {#each modelInfo.warnings as warning}
                    <p>{warning}</p>
                  {/each}
                </div>
              {:else if modelInfo}
                <div class="grid gap-2 text-txtsecondary md:grid-cols-2">
                  <p>{modelInfo.architecture || modelInfo.modelType || $tx.editor.modelInfo.unknown} {modelInfo.quantization ? `· ${modelInfo.quantization}` : ""}</p>
                  <p>{$tx.editor.modelInfo.context}: {numberOrUnknown(modelInfo.contextLength)}</p>
                  <p>{$tx.editor.modelInfo.layers}: {numberOrUnknown(modelInfo.blockCount)}</p>
                  <p>{$tx.editor.modelInfo.heads}: {numberOrUnknown(modelInfo.headCount)} / KV {numberOrUnknown(modelInfo.headCountKV)}</p>
                  <p>{$tx.editor.modelInfo.vocab}: {numberOrUnknown(modelInfo.vocabularySize)}</p>
                </div>
              {:else}
                <p class="text-txtsecondary">{runtimeKind === "mlx-lm" ? $tx.editor.help.noMLXInfo : $tx.editor.help.noModelLimits}</p>
              {/if}
            </div>
          {/if}

          {#if runtimeKind === "llama-server"}
            <div class="grid gap-4 md:grid-cols-2">
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.executable}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.executable} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.modelPath}</span>
                <input
                  class="w-full rounded border border-border bg-card px-3 py-2"
                  bind:value={runtime.modelPath}
                  onchange={() => inspectModelPath(runtime.modelPath)}
                />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.host}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.host} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.port}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.port} />
              </label>
              <SliderNumber label={$tx.editor.fields.ctxSize} bind:value={runtime.ctxSize} min={512} max={contextMax} step={512} help={maxHelp($tx.editor.fields.ctxSize, contextMax)} />
              <SliderNumber label={$tx.editor.fields.threads} bind:value={runtime.threads} min={1} max={threadsMax} step={1} help={maxHelp($tx.editor.fields.threads, threadsMax)} />
              <SliderNumber label={$tx.editor.fields.threadsBatch} bind:value={runtime.threadsBatch} min={1} max={threadsMax} step={1} help={maxHelp($tx.editor.fields.threadsBatch, threadsMax)} />
              <SliderNumber label={$tx.editor.fields.batchSize} bind:value={runtime.batchSize} min={32} max={batchMax} step={32} help={maxHelp($tx.editor.fields.batchSize, batchMax)} />
              <SliderNumber label={$tx.editor.fields.ubatchSize} bind:value={runtime.ubatchSize} min={32} max={microBatchMax} step={32} help={maxHelp($tx.editor.fields.ubatchSize, microBatchMax)} />
              <SliderNumber label={$tx.editor.fields.parallel} bind:value={runtime.parallel} min={1} max={parallelMax} step={1} help={maxHelp($tx.editor.fields.parallel, parallelMax)} />
              <SliderNumber label={$tx.editor.fields.priority} bind:value={runtime.priority} min={-3} max={3} step={1} help="-3..3" />
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.device}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" placeholder="none / auto / Metal" bind:value={runtime.device} />
              </label>
              <SliderNumber label={$tx.editor.fields.gpuLayers} bind:value={runtime.gpuLayers} min={0} max={gpuLayerMax} step={1} help={maxHelp($tx.editor.fields.gpuLayers, gpuLayerMax)} />
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.flashAttn}</span>
                <select class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.flashAttention}>
                  <option value=""></option>
                  <option value="auto">auto</option>
                  <option value="on">on</option>
                  <option value="off">off</option>
                </select>
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.cacheTypeK}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.cacheTypeK} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.cacheTypeV}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.cacheTypeV} />
              </label>
              <label class="mt-7 flex items-center gap-2">
                <input type="checkbox" bind:checked={runtime.noWarmup} />
                <span>{$tx.editor.fields.noWarmup}</span>
              </label>
              <label class="block md:col-span-2">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.extraArgs}</span>
                <textarea class="min-h-20 w-full rounded border border-border bg-card px-3 py-2 font-mono text-sm" bind:value={runtime.extraArgs}></textarea>
              </label>
            </div>
          {:else if runtimeKind === "mlx-lm"}
            <div class="grid gap-4 md:grid-cols-2">
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.executable}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={mlxRuntime.executable} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.modelPath}</span>
                <input
                  class="w-full rounded border border-border bg-card px-3 py-2"
                  bind:value={mlxRuntime.modelPath}
                  onchange={() => inspectModelPath(mlxRuntime.modelPath)}
                />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.host}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={mlxRuntime.host} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.port}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={mlxRuntime.port} />
              </label>
              <SliderNumber label={$tx.editor.fields.maxTokens} bind:value={mlxRuntime.maxTokens} min={1} max={Math.max(contextMax || 8192, 8192)} step={128} help={`${$tx.editor.fields.maxTokens}: <= ${Math.max(contextMax || 8192, 8192).toLocaleString()}`} />
              <SliderNumber label={$tx.editor.fields.temp} bind:value={mlxRuntime.temp} min={0} max={2} step={0.05} allowEmpty={false} />
              <SliderNumber label={$tx.editor.fields.topP} bind:value={mlxRuntime.topP} min={0} max={1} step={0.05} allowEmpty={false} />
              <SliderNumber label={$tx.editor.fields.topK} bind:value={mlxRuntime.topK} min={0} max={200} step={1} />
              <SliderNumber label={$tx.editor.fields.minP} bind:value={mlxRuntime.minP} min={0} max={1} step={0.01} />
              <SliderNumber label={$tx.editor.fields.numDraftTokens} bind:value={mlxRuntime.numDraftTokens} min={1} max={16} step={1} />
              <SliderNumber label={$tx.editor.fields.decodeConcurrency} bind:value={mlxRuntime.decodeConcurrency} min={1} max={64} step={1} />
              <SliderNumber label={$tx.editor.fields.promptConcurrency} bind:value={mlxRuntime.promptConcurrency} min={1} max={32} step={1} />
              <SliderNumber label={$tx.editor.fields.prefillStepSize} bind:value={mlxRuntime.prefillStepSize} min={128} max={Math.max(contextMax || 8192, 8192)} step={128} />
              <SliderNumber label={$tx.editor.fields.promptCacheSize} bind:value={mlxRuntime.promptCacheSize} min={0} max={64} step={1} />
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.promptCacheBytes}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" placeholder="4G" bind:value={mlxRuntime.promptCacheBytes} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.logLevel}</span>
                <select class="w-full rounded border border-border bg-card px-3 py-2" bind:value={mlxRuntime.logLevel}>
                  <option value=""></option>
                  <option value="DEBUG">DEBUG</option>
                  <option value="INFO">INFO</option>
                  <option value="WARNING">WARNING</option>
                  <option value="ERROR">ERROR</option>
                  <option value="CRITICAL">CRITICAL</option>
                </select>
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.allowedOrigins}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" placeholder="*" bind:value={mlxRuntime.allowedOrigins} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.adapterPath}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={mlxRuntime.adapterPath} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.draftModel}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={mlxRuntime.draftModel} />
              </label>
              <label class="block md:col-span-2">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.chatTemplate}</span>
                <textarea class="min-h-20 w-full rounded border border-border bg-card px-3 py-2 font-mono text-sm" bind:value={mlxRuntime.chatTemplate}></textarea>
              </label>
              <label class="block md:col-span-2">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.chatTemplateArgs}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2 font-mono text-sm" placeholder={'{"enable_thinking":false}'} bind:value={mlxRuntime.chatTemplateArgs} />
              </label>
              <label class="flex items-center gap-2">
                <input type="checkbox" bind:checked={mlxRuntime.trustRemoteCode} />
                <span>{$tx.editor.fields.trustRemoteCode}</span>
              </label>
              <label class="flex items-center gap-2">
                <input type="checkbox" bind:checked={mlxRuntime.useDefaultChatTemplate} />
                <span>{$tx.editor.fields.useDefaultChatTemplate}</span>
              </label>
              <label class="flex items-center gap-2">
                <input type="checkbox" bind:checked={mlxRuntime.pipeline} />
                <span>{$tx.editor.fields.pipeline}</span>
              </label>
              <label class="block md:col-span-2">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.extraArgs}</span>
                <textarea class="min-h-20 w-full rounded border border-border bg-card px-3 py-2 font-mono text-sm" bind:value={mlxRuntime.extraArgs}></textarea>
              </label>
            </div>
          {:else}
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.cmd}</span>
              <textarea class="min-h-56 w-full rounded border border-border bg-card px-3 py-2 font-mono text-sm" bind:value={rawCommand}></textarea>
            </label>
          {/if}

          <div class="mt-4">
            <h3 class="p-0 text-base">{$tx.editor.preview}</h3>
            <pre class="mt-2 max-h-48 overflow-auto rounded bg-background p-3 text-xs"><code>{commandPreview}</code></pre>
          </div>
        {:else if activeTab === "requests"}
          <div class="grid gap-4">
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.stripParams}</span>
              <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={config.filters.stripParams} />
              <span class="mt-1 block text-xs text-txtsecondary">{$tx.editor.help.stripParams}</span>
            </label>
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.setParams}</span>
              <textarea class="min-h-44 w-full rounded border border-border bg-card px-3 py-2 font-mono text-sm" bind:value={setParamsText}></textarea>
              <span class="mt-1 block text-xs text-txtsecondary">{$tx.editor.help.setParams}</span>
            </label>
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.setParamsByID}</span>
              <textarea class="min-h-44 w-full rounded border border-border bg-card px-3 py-2 font-mono text-sm" bind:value={setParamsByIDText}></textarea>
            </label>
          </div>
        {:else}
          <div class="grid gap-4 md:grid-cols-2">
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.useModelName}</span>
              <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={config.useModelName} />
            </label>
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.proxy}</span>
              <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={config.proxy} />
            </label>
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.checkEndpoint}</span>
              <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={config.checkEndpoint} />
            </label>
            <SliderNumber label={$tx.editor.fields.concurrencyLimit} bind:value={config.concurrencyLimit} min={0} max={64} step={1} allowEmpty={false} />
            <label class="block md:col-span-2">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.cmdStop}</span>
              <input class="w-full rounded border border-border bg-card px-3 py-2 font-mono text-sm" bind:value={config.cmdStop} />
            </label>
            <label class="block md:col-span-2">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.env}</span>
              <textarea class="min-h-28 w-full rounded border border-border bg-card px-3 py-2 font-mono text-sm" bind:value={envText}></textarea>
              <span class="mt-1 block text-xs text-txtsecondary">{$tx.editor.help.env}</span>
            </label>
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.sendLoadingState}</span>
              <select
                class="w-full rounded border border-border bg-card px-3 py-2"
                value={config.sendLoadingState === null || config.sendLoadingState === undefined ? "inherit" : String(config.sendLoadingState)}
                onchange={handleSendLoadingStateChange}
              >
                <option value="inherit">{$tx.editor.fields.inherit}</option>
                <option value="true">true</option>
                <option value="false">false</option>
              </select>
            </label>
          </div>
        {/if}
      </div>

      <div class="shrink-0 border-t border-card-border p-4">
        <div class="flex flex-wrap justify-end gap-2">
          <button class="btn" onclick={validate} disabled={isValidating || isSaving}>
            {isValidating ? $tx.common.validating : $tx.common.validate}
          </button>
          <button class="btn bg-primary text-btn-primary-text" onclick={save} disabled={isSaving || isValidating}>
            {isSaving ? $tx.common.saving : $tx.common.save}
          </button>
        </div>
      </div>
    {:else}
      <div class="flex-1 p-4 text-error">{error || $tx.editor.loadingFailed}</div>
    {/if}
  </aside>
{/if}
