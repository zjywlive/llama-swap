<script lang="ts">
  import { proxyLogs, upstreamLogs } from "../stores/api";
  import { screenWidth } from "../stores/theme";
  import { persistentStore } from "../stores/persistent";
  import LogPanel from "../components/LogPanel.svelte";
  import ResizablePanels from "../components/ResizablePanels.svelte";
  import { tx } from "../stores/i18n";

  type ViewMode = "proxy" | "upstream" | "panels";

  const viewModeStore = persistentStore<ViewMode>("logviewer-view-mode", "panels");

  let direction = $derived<"horizontal" | "vertical">(
    $screenWidth === "xs" || $screenWidth === "sm" ? "vertical" : "horizontal",
  );
</script>

<div class="flex flex-col h-full w-full gap-2">
  <div class="flex items-center gap-1">
    <button
      onclick={() => viewModeStore.set("panels")}
      class:btn={true}
      class:bg-primary={$viewModeStore === "panels"}
      class:text-btn-primary-text={$viewModeStore === "panels"}
    >
      {$tx.logs.both}
    </button>
    <button
      onclick={() => viewModeStore.set("proxy")}
      class:btn={true}
      class:bg-primary={$viewModeStore === "proxy"}
      class:text-btn-primary-text={$viewModeStore === "proxy"}
    >
      {$tx.logs.proxy}
    </button>
    <button
      onclick={() => viewModeStore.set("upstream")}
      class:btn={true}
      class:bg-primary={$viewModeStore === "upstream"}
      class:text-btn-primary-text={$viewModeStore === "upstream"}
    >
      {$tx.logs.upstream}
    </button>
  </div>

  <div class="flex-1 w-full overflow-hidden">
    {#if $viewModeStore === "panels"}
      <ResizablePanels {direction} storageKey="logviewer-panel-group">
        {#snippet leftPanel()}
          <LogPanel id="proxy" title={$tx.logs.proxyLogs} logData={$proxyLogs} />
        {/snippet}
        {#snippet rightPanel()}
          <LogPanel id="upstream" title={$tx.logs.upstreamLogs} logData={$upstreamLogs} />
        {/snippet}
      </ResizablePanels>
    {:else if $viewModeStore === "proxy"}
      <LogPanel id="proxy" title={$tx.logs.proxyLogs} logData={$proxyLogs} />
    {:else}
      <LogPanel id="upstream" title={$tx.logs.upstreamLogs} logData={$upstreamLogs} />
    {/if}
  </div>
</div>
