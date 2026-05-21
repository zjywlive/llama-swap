<script lang="ts">
  import { createEditableModel, fetchEditableModels, scanLocalModels } from "../stores/api";
  import { tx } from "../stores/i18n";
  import type { EditableModelConfig, ScannedGGUFModel } from "../lib/types";
  import { buildLlamaServerCommand, defaultRuntime, parseRuntimeCommand } from "../lib/modelConfig";

  interface Props {
    open: boolean;
    onClose: () => void;
    onImported?: (modelId: string) => void;
  }

  let { open, onClose, onImported }: Props = $props();

  let directory = $state("");
  let isScanning = $state(false);
  let isImporting = $state(false);
  let recursive = $state(true);
  let models = $state<ScannedGGUFModel[]>([]);
  let warnings = $state<string[]>([]);
  let error = $state<string | null>(null);
  let message = $state<string | null>(null);
  let inferredMessage = $state<string | null>(null);
  let importingPath = $state<string | null>(null);
  let wasOpen = $state(false);

  $effect(() => {
    if (open && !wasOpen) {
      error = null;
      message = null;
      inferredMessage = null;
      void inferDefaultDirectory();
    }
    wasOpen = open;
  });

  function dirname(path: string): string {
    const normalized = path.trim().replace(/[\\/]+$/, "");
    const slash = Math.max(normalized.lastIndexOf("/"), normalized.lastIndexOf("\\"));
    return slash > 0 ? normalized.slice(0, slash) : "";
  }

  function basenameWithoutExt(path: string): string {
    const base = path.split(/[\\/]/).pop() || path;
    return base.replace(/\.[^.]+$/, "");
  }

  function numberLabel(value: number | undefined): string {
    return value && value > 0 ? value.toLocaleString() : $tx.common.unknown;
  }

  function limitedDefault(preferred: number, max: number | undefined, min = 1): number {
    const limit = max && max > 0 ? max : preferred;
    return Math.max(min, Math.min(preferred, limit));
  }

  async function inferDefaultDirectory(): Promise<void> {
    if (directory.trim()) return;
    try {
      const response = await fetchEditableModels();
      for (const model of response.models) {
        let modelPath = model.modelInfo?.path ?? "";
        if (!modelPath) {
          const parsed = parseRuntimeCommand(model.cmd);
          modelPath = parsed.runtime.modelPath;
        }
        if (!modelPath || modelPath.includes("${")) continue;
        const parent = dirname(modelPath);
        const grandParent = dirname(parent);
        directory = grandParent || parent;
        inferredMessage = directory ? `${$tx.importer.detected}: ${directory}` : null;
        return;
      }
      inferredMessage = $tx.importer.inferFailed;
    } catch {
      inferredMessage = $tx.importer.inferFailed;
    }
  }

  async function scan(): Promise<void> {
    isScanning = true;
    error = null;
    message = null;
    warnings = [];
    models = [];
    try {
      const response = await scanLocalModels(directory, recursive);
      directory = response.dir;
      models = response.models ?? [];
      warnings = response.warnings ?? [];
      if (models.length === 0) {
        message = $tx.importer.noModels;
      }
    } catch (err) {
      error = `${$tx.importer.scanFailed}: ${err instanceof Error ? err.message : String(err)}`;
    } finally {
      isScanning = false;
    }
  }

  function buildImportedConfig(scanned: ScannedGGUFModel): EditableModelConfig {
    const info = scanned.modelInfo;
    const limits = info?.limits;
    const hardwareThreads = typeof navigator !== "undefined" && navigator.hardwareConcurrency ? navigator.hardwareConcurrency : 4;
    const ctxSize = limitedDefault(4096, limits?.contextMax || info?.contextLength, 512);
    const batchSize = limitedDefault(512, Math.min(limits?.batchMax || ctxSize, ctxSize), 32);
    const microBatchSize = limitedDefault(256, Math.min(limits?.microBatchMax || batchSize, batchSize), 32);
    const threads = limitedDefault(4, Math.min(limits?.threadsMax || hardwareThreads, hardwareThreads), 1);
    const runtime = defaultRuntime();

    runtime.modelPath = scanned.path;
    runtime.ctxSize = ctxSize;
    runtime.threads = threads;
    runtime.threadsBatch = threads;
    runtime.batchSize = batchSize;
    runtime.ubatchSize = microBatchSize;
    runtime.parallel = 1;
    runtime.priority = -1;
    runtime.gpuLayers = 0;
    runtime.noWarmup = true;
    runtime.extraArgs = `--alias ${scanned.idSuggestion}`;

    return {
      id: scanned.idSuggestion,
      cmd: buildLlamaServerCommand(runtime),
      cmdStop: "",
      name: scanned.name || basenameWithoutExt(scanned.path),
      description: "",
      env: [],
      proxy: "http://127.0.0.1:${PORT}",
      aliases: [],
      checkEndpoint: "/health",
      ttl: 300,
      unlisted: false,
      useModelName: "",
      concurrencyLimit: 0,
      sendLoadingState: null,
      filters: {
        stripParams: "",
        setParams: {},
        setParamsByID: {},
      },
      metadata: {},
      timeouts: {
        connect: 30,
        keepalive: 30,
        responseHeader: 0,
        tlsHandshake: 10,
        expectContinue: 1,
        idleConn: 90,
      },
      modelInfo: info ?? null,
    };
  }

  async function importModel(scanned: ScannedGGUFModel): Promise<void> {
    isImporting = true;
    importingPath = scanned.path;
    error = null;
    message = null;
    try {
      const config = buildImportedConfig(scanned);
      await createEditableModel(config);
      models = models.map((item) =>
        item.path === scanned.path
          ? { ...item, imported: true, existingId: config.id, idSuggestion: config.id }
          : item
      );
      message = $tx.importer.importedMessage;
      onImported?.(config.id);
    } catch (err) {
      error = `${$tx.importer.importFailed}: ${err instanceof Error ? err.message : String(err)}`;
    } finally {
      importingPath = null;
      isImporting = false;
    }
  }
</script>

{#if open}
  <div class="fixed inset-0 z-40 bg-black/30" role="button" tabindex="0" aria-label={$tx.common.close} onclick={onClose} onkeydown={(e) => e.key === "Escape" && onClose()}></div>
  <aside class="fixed right-0 top-0 z-50 flex h-screen w-full max-w-4xl flex-col border-l border-card-border bg-surface shadow-xl">
    <div class="shrink-0 border-b border-card-border p-4">
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0">
          <h2 class="p-0 text-2xl">{$tx.importer.title}</h2>
          <p class="mt-1 text-sm text-txtsecondary">{$tx.importer.subtitle}</p>
        </div>
        <button class="btn btn--sm" onclick={onClose}>{$tx.common.close}</button>
      </div>
    </div>

    <div class="flex-1 overflow-y-auto p-4">
      {#if error}
        <div class="mb-4 rounded border border-error/30 bg-error/10 p-3 text-sm text-error">{error}</div>
      {/if}
      {#if message}
        <div class="mb-4 rounded border border-success/30 bg-success/10 p-3 text-sm text-success">{message}</div>
      {/if}

      <div class="grid gap-3 md:grid-cols-[1fr_auto] md:items-end">
        <label class="block">
          <span class="mb-1 block text-sm font-medium">{$tx.importer.directory}</span>
          <input class="w-full rounded border border-border bg-card px-3 py-2 font-mono text-sm" bind:value={directory} placeholder="/Users/rick/Developer/Local_Ai/models" />
          <span class="mt-1 block text-xs text-txtsecondary">{$tx.importer.directoryHelp}</span>
          {#if inferredMessage}
            <span class="mt-1 block break-all text-xs text-txtsecondary">{inferredMessage}</span>
          {/if}
        </label>
        <div class="flex flex-wrap items-center gap-3">
          <label class="flex items-center gap-2">
            <input type="checkbox" bind:checked={recursive} />
            <span>{$tx.importer.recursive}</span>
          </label>
          <button class="btn bg-primary text-btn-primary-text" onclick={scan} disabled={isScanning || !directory.trim()}>
            {isScanning ? $tx.importer.scanning : $tx.importer.scan}
          </button>
        </div>
      </div>

      {#if warnings.length > 0}
        <div class="mt-4 rounded border border-yellow-400/40 bg-yellow-500/10 p-3 text-sm text-yellow-700 dark:text-yellow-200">
          <p class="font-medium">{$tx.importer.warnings}</p>
          {#each warnings as warning}
            <p class="break-all">{warning}</p>
          {/each}
        </div>
      {/if}

      {#if models.length > 0}
        <div class="mt-4 overflow-x-auto">
          <table class="w-full">
            <thead class="sticky top-0 bg-card z-10">
              <tr class="text-left border-b border-gray-200 dark:border-white/10 bg-surface">
                <th>{$tx.models.name}</th>
                <th>{$tx.importer.modelId}</th>
                <th>{$tx.importer.context}</th>
                <th>{$tx.importer.layers}</th>
                <th>{$tx.models.actions}</th>
              </tr>
            </thead>
            <tbody>
              {#each models as model (model.path)}
                <tr class="border-b border-gray-200 hover:bg-secondary-hover">
                  <td class="max-w-sm py-2">
                    <div class="font-semibold">{model.name || basenameWithoutExt(model.path)}</div>
                    <div class="mt-1 break-all font-mono text-xs text-txtsecondary">{model.path}</div>
                    {#if model.modelInfo}
                      <div class="mt-1 text-xs text-txtsecondary">
                        {model.modelInfo.architecture || $tx.common.unknown}
                        {model.modelInfo.quantization ? ` · ${model.modelInfo.quantization}` : ""}
                      </div>
                    {/if}
                    {#if model.warnings?.length}
                      <div class="mt-1 text-xs text-yellow-700 dark:text-yellow-200">
                        {#each model.warnings as warning}
                          <p class="break-all">{warning}</p>
                        {/each}
                      </div>
                    {/if}
                  </td>
                  <td class="break-all font-mono text-xs">{model.idSuggestion}</td>
                  <td>{numberLabel(model.modelInfo?.contextLength)}</td>
                  <td>{numberLabel(model.modelInfo?.blockCount)}</td>
                  <td class="whitespace-nowrap">
                    {#if model.imported}
                      <span class="status status--ready">{model.existingId ? `${$tx.importer.alreadyImported}: ${model.existingId}` : $tx.importer.imported}</span>
                    {:else}
                      <button class="btn btn--sm" onclick={() => importModel(model)} disabled={isImporting || model.warnings?.length > 0}>
                        {importingPath === model.path ? $tx.importer.importing : $tx.importer.import}
                      </button>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  </aside>
{/if}
