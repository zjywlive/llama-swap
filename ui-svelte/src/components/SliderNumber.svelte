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

  let sliderValue = $derived(value === "" ? min : clamp(Number(value)));

  $effect(() => {
    if (value !== "") {
      const next = clamp(Number(value));
      if (next !== value) value = next;
    }
  });

  function clamp(next: number): number {
    if (!Number.isFinite(next)) return min;
    return Math.min(max, Math.max(min, next));
  }

  function setNumber(raw: string): void {
    if (allowEmpty && raw.trim() === "") {
      value = "";
      return;
    }
    value = clamp(Number(raw));
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
      value={value}
      oninput={(event) => setNumber((event.currentTarget as HTMLInputElement).value)}
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
    oninput={(event) => (value = clamp(Number((event.currentTarget as HTMLInputElement).value)))}
  />
  {#if help}
    <span class="mt-1 block text-xs text-txtsecondary">{help}</span>
  {/if}
</label>
