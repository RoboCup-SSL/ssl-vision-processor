<script lang="ts">
  import { Alert, Input, P, Select, Toggle } from "flowbite-svelte";
  import SettingsCard from "../SettingsCard.svelte";
  import FormRow from "../FormRow.svelte";
  import RichText from "../RichText.svelte";
  import { network } from "../network.svelte";
  import {
    COLORS,
    DEFAULT_ROBOT_HEIGHT,
    choice,
    isDefaultHeightIssue,
    heightIssue,
    setCustomHeight,
    setFromTable,
    setTeam,
    slotFor,
    teamName,
    type Color,
  } from "../teams.svelte";
  import { overview as text } from "../text/overview";
  import { preferences } from "../preferences.svelte";

  // Select value for "Custom", which can't be a team name in the table.
  const CUSTOM = "\u0000custom";

  const LABEL: Record<Color, string> = {
    yellow: text.teams.yellow,
    blue: text.teams.blue,
  };
  const SWATCH: Record<Color, string> = {
    yellow: "bg-team-yellow",
    blue: "bg-team-blue",
  };

  let ref = $derived(network.state?.referee);
  let tableTeams = $derived(
    Object.keys(ref?.heights ?? {}).sort((a, b) => a.localeCompare(b)),
  );

  function tableHeight(name: string): string {
    const h = ref?.heights?.[name];

    return h === undefined ? text.teams.notInTable : text.teams.height(h);
  }
</script>

<div class="max-w-5xl">
  <SettingsCard title={text.teams.title}>
    <Alert color="primary" class="mb-2 p-2 text-sm"
      >{text.teams.notApplied}</Alert
    >

    <div class="sides">
      {#each COLORS as color (color)}
        {@const team = teamName(color)}
        {@const slot = slotFor(color)}
        {@const own = choice(slot)}
        {@const issue = heightIssue(color)}
        <div class="side">
          <FormRow
            label={LABEL[color]}
            for={`team-${color}-gc`}
            notes={text.teams.fromGameControllerNotes}
          >
            <span class="flex flex-wrap items-center gap-2">
              <span class={`inline-block h-3 w-3 rounded-full ${SWATCH[color]}`}
              ></span>
              <span
                class="font-medium"
                class:text-gray-500={!team}
                class:dark:text-gray-400={!team}
                >{team || text.teams.noGameController}</span
              >
              <Toggle
                id={`team-${color}-gc`}
                size="small"
                checked={own === undefined}
                disabled={!slot}
                onchange={(e: Event) => {
                  if (slot)
                    setFromTable(
                      slot,
                      (e.currentTarget as HTMLInputElement).checked,
                    );
                }}
                ><span class="text-sm">{text.teams.fromGameController}</span
                ></Toggle
              >
            </span>
          </FormRow>

          <FormRow
            label={text.teams.team}
            for={`team-${color}-name`}
            warnings={issue &&
            (!isDefaultHeightIssue(issue) || preferences.tooltipsEnabled)
              ? [text.teams.issue[issue]]
              : []}
          >
            {#if !slot}
              <span class="text-sm text-gray-500 dark:text-gray-400"
                >{text.teams.waiting}</span
              >
            {:else if own === undefined}
              <span class="text-sm text-gray-600 dark:text-gray-400"
                >{team && ref?.heights?.[team] !== undefined
                  ? tableHeight(team)
                  : text.teams.defaultHeight(DEFAULT_ROBOT_HEIGHT)}</span
              >
            {:else}
              <span class="flex items-center gap-2">
                <div class="w-52 shrink-0">
                  <Select
                    id={`team-${color}-name`}
                    size="sm"
                    placeholder=""
                    value={own.name ?? CUSTOM}
                    onchange={(e: Event) => {
                      const value = (e.currentTarget as HTMLSelectElement)
                        .value;
                      if (value === CUSTOM) setFromTable(slot, false);
                      else setTeam(slot, value);
                    }}
                  >
                    <option value={CUSTOM}>{text.teams.custom}</option>
                    {#each tableTeams as name (name)}
                      <option value={name}>{name}</option>
                    {/each}
                  </Select>
                </div>
                {#if own.name !== undefined}
                  <span class="text-sm text-gray-600 dark:text-gray-400"
                    >{tableHeight(own.name)}</span
                  >
                {:else}
                  <!-- Custom: the height goes right beside the selector. -->
                  <div class="w-24 shrink-0">
                    <Input
                      id={`team-${color}-height`}
                      aria-label={text.teams.customHeight}
                      type="number"
                      size="sm"
                      min="1"
                      max="500"
                      step="1"
                      value={own.height ?? ""}
                      onchange={(e: Event) => {
                        const v = (e.currentTarget as HTMLInputElement)
                          .valueAsNumber;
                        if (Number.isFinite(v) && v > 0)
                          setCustomHeight(slot, v);
                      }}
                    />
                  </div>
                  <span class="text-sm text-gray-500 dark:text-gray-400"
                    >mm</span
                  >
                {/if}
              </span>
            {/if}
          </FormRow>
        </div>
      {/each}
    </div>

    {#if ref}
      <P size="xs" class="mt-1 text-gray-500 dark:text-gray-400">
        <RichText
          text={ref.heightsError
            ? text.teams.tableError(ref.heightsFile, ref.heightsError)
            : text.teams.table(
                ref.heightsFile,
                DEFAULT_ROBOT_HEIGHT,
                ref.maxHeight ?? 0,
              )}
        />
      </P>
    {/if}
  </SettingsCard>
</div>

<style>
  /* Yellow and blue side by side, stacked when there's no room. */
  .sides {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(22rem, 1fr));
    gap: 0.5rem 1.5rem;
  }

  .side + .side {
    padding-left: 1.5rem;
    border-left: 1px solid var(--line);
  }
</style>
