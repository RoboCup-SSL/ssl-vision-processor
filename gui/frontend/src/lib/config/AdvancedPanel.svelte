<script lang="ts">
  import { Alert, Heading, P } from "flowbite-svelte";
  import SettingsCard from "../SettingsCard.svelte";
  import SliderField from "./SliderField.svelte";
  import RichText from "../RichText.svelte";
  import {
    ADVANCED,
    advancedOverrides,
    advancedValue,
    setAdvanced,
    type AdvancedSetting,
  } from "../advanced.svelte";
  import { advanced as text } from "../text/advanced";

  const CARDS = [
    { id: "blobs", title: text.cards.blobs },
    { id: "tolerances", title: text.cards.tolerances },
    { id: "tracking", title: text.cards.tracking },
  ] as const;

  let overriding = $derived(advancedOverrides());

  function fieldText(s: AdvancedSetting): { label: string; notes: string[] } {
    return text.fields[s.key as keyof typeof text.fields];
  }
</script>

<div class="max-w-3xl">
  <Heading tag="h2" class="mb-2 text-xl font-semibold">{text.heading}</Heading>
  <P size="sm" class="mb-3 text-gray-600 dark:text-gray-400"
    ><RichText text={text.intro} /></P
  >

  {#if overriding.length > 0}
    <Alert color="yellow" class="mb-3 p-2 text-sm"
      >{text.overriding(overriding)}</Alert
    >
  {/if}

  {#each CARDS as card (card.id)}
    <SettingsCard title={card.title}>
      {#each ADVANCED[card.id] as setting (setting.key)}
        {@const own = advancedValue(setting)}
        {@const label = fieldText(setting)}
        <SliderField
          id={`advanced-${setting.key}`}
          label={label.label}
          value={own ?? setting.fallback}
          min={setting.min}
          max={setting.max}
          step={setting.step}
          unit={setting.unit}
          notes={[...label.notes, text.defaultIs(String(setting.fallback))]}
          hints={setting.restart ? [text.restart] : []}
          reset={{
            label: text.reset,
            disabled: own === undefined,
            onclick: () => {
              setAdvanced(setting, undefined);
            },
          }}
          onchange={(value: number) => {
            setAdvanced(setting, value);
          }}
        />
      {/each}
    </SettingsCard>
  {/each}
</div>
