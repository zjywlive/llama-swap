<script lang="ts">
  import { fetchEditableModel, saveEditableModel, validateEditableModel } from "../stores/api";
  import { tx } from "../stores/i18n";
  import type { EditableModelConfig } from "../lib/types";
  import { buildLlamaServerCommand, defaultRuntime, parseRuntimeCommand, type LlamaServerRuntime } from "../lib/modelConfig";

  interface Props {
    modelId: string | null;
    onClose: () => void;
  }

  let { modelId, onClose }: Props = $props();

  type Tab = "basic" | "runtime" | "requests" | "advanced";

  let activeTab = $state<Tab>("basic");
  let loadedModelId = $state<string | null>(null);
  let config = $state<EditableModelConfig | null>(null);
  let runtimeKind = $state<"llama-server" | "raw">("raw");
  let runtime = $state<LlamaServerRuntime>(defaultRuntime());
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

  let commandPreview = $derived(runtimeKind === "llama-server" ? buildLlamaServerCommand(runtime) : rawCommand);

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
      rawCommand = loaded.cmd;
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
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.ttl}</span>
              <input type="number" min="0" class="w-full rounded border border-border bg-card px-3 py-2" bind:value={config.ttl} />
              <span class="mt-1 block text-xs text-txtsecondary">{$tx.editor.help.ttl}</span>
            </label>
            <label class="mt-7 flex items-center gap-2">
              <input type="checkbox" bind:checked={config.unlisted} />
              <span>{$tx.editor.fields.unlisted}</span>
            </label>
          </div>
        {:else if activeTab === "runtime"}
          <div class="mb-4 rounded border border-border bg-card p-3 text-sm text-txtsecondary">
            {runtimeKind === "llama-server" ? $tx.editor.help.runtimeAuto : $tx.editor.help.runtimeRaw}
          </div>

          {#if runtimeKind === "llama-server"}
            <div class="grid gap-4 md:grid-cols-2">
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.executable}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.executable} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.modelPath}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.modelPath} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.host}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.host} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.port}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.port} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.ctxSize}</span>
                <input type="number" min="0" class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.ctxSize} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.threads}</span>
                <input type="number" class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.threads} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.threadsBatch}</span>
                <input type="number" class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.threadsBatch} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.batchSize}</span>
                <input type="number" class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.batchSize} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.ubatchSize}</span>
                <input type="number" class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.ubatchSize} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.parallel}</span>
                <input type="number" class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.parallel} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.priority}</span>
                <input type="number" class="w-full rounded border border-border bg-card px-3 py-2" bind:value={runtime.priority} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.device}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" placeholder="none / auto / Metal" bind:value={runtime.device} />
              </label>
              <label class="block">
                <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.gpuLayers}</span>
                <input class="w-full rounded border border-border bg-card px-3 py-2" placeholder="auto / all / 0 / 99" bind:value={runtime.gpuLayers} />
              </label>
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
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{$tx.editor.fields.concurrencyLimit}</span>
              <input type="number" min="0" class="w-full rounded border border-border bg-card px-3 py-2" bind:value={config.concurrencyLimit} />
            </label>
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
