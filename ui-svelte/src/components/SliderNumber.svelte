<script lang="ts">
  interface Props {
    label: string;
    value: number | "";
    min?: number;
    max: number;
    step?: number;
    help?: string;
    disabled?: boolean;
    allowEmpty?: boolean;
  }

  let {
    label,
    value = $bindable<number | "">(""),
    min = 0,
    max,
    step = 1,
    help = "",
    disabled = false,
    allowEmpty = true,
  }: Props = $props();

  let editing = $state(false);
  let inputText = $state("");
  let precision = $derived(Math.min(12, Math.max(decimalPlaces(step), decimalPlaces(min))));
  let sliderValue = $derived(value === "" ? min : normalize(Number(value)));

  $effect(() => {
    if (value !== "") {
      const next = normalize(Number(value));
      if (next !== value) value = next;
    }
    if (!editing) {
      inputText = value === "" ? "" : formatValue(value);
    }
  });

  function decimalPlaces(next: number): number {
    const text = String(next);
    if (text.includes("e-")) return Number(text.split("e-")[1]) || 0;
    const dot = text.indexOf(".");
    return dot === -1 ? 0 : text.length - dot - 1;
  }

  function clamp(next: number): number {
    if (!Number.isFinite(next)) return min;
    return Math.min(max, Math.max(min, next));
  }

  function normalize(next: number): number {
    const bounded = clamp(next);
    if (!Number.isFinite(step) || step <= 0) return bounded;
    const snapped = min + Math.round((bounded - min) / step) * step;
    const factor = 10 ** precision;
    return clamp(Math.round(snapped * factor) / factor);
  }

  function formatValue(next: number | ""): string {
    return next === "" ? "" : String(next);
  }

  function resetInput(): void {
    editing = false;
    inputText = value === "" ? "" : formatValue(value);
  }

  function commitInput(): void {
    const raw = inputText.trim();
    editing = false;

    if (raw === "") {
      if (allowEmpty) {
        value = "";
        inputText = "";
      } else {
        resetInput();
      }
      return;
    }

    const parsed = Number(raw);
    if (!Number.isFinite(parsed)) {
      resetInput();
      return;
    }

    value = normalize(parsed);
    inputText = formatValue(value);
  }

  function handleNumberKeydown(event: KeyboardEvent): void {
    if (event.key === "Enter") {
      commitInput();
      (event.currentTarget as HTMLInputElement).blur();
    } else if (event.key === "Escape") {
      resetInput();
      (event.currentTarget as HTMLInputElement).blur();
    }
  }

  function setSlider(raw: string): void {
    editing = false;
    value = normalize(Number(raw));
  }
</script>

<label class="block">
  <div class="mb-1 flex items-baseline justify-between gap-3">
    <span class="text-sm font-medium">{label}</span>
    <input
      type="number"
      class="w-24 rounded border border-border bg-card px-2 py-1 text-right text-sm"
      {min}
      {max}
      {step}
      {disabled}
      value={inputText}
      onfocus={() => (editing = true)}
      oninput={(event) => {
        editing = true;
        inputText = (event.currentTarget as HTMLInputElement).value;
      }}
      onblur={commitInput}
      onkeydown={handleNumberKeydown}
    />
  </div>
  <input
    type="range"
    class="w-full accent-primary"
    {min}
    {max}
    {step}
    {disabled}
    value={sliderValue}
    oninput={(event) => setSlider((event.currentTarget as HTMLInputElement).value)}
  />
  {#if help}
    <span class="mt-1 block text-xs text-txtsecondary">{help}</span>
  {/if}
</label>
