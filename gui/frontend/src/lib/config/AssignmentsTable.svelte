<script lang="ts">
  import {
    Table,
    TableBody,
    TableBodyCell,
    TableBodyRow,
    TableHead,
    TableHeadCell,
  } from "flowbite-svelte";
  import SettingsCard from "../SettingsCard.svelte";
  import { cameraStatus, type ConfigDocument } from "../config.svelte";
  import {
    cameraDevice,
    deviceLabel,
    hostColor,
    hostName,
    liveLabel,
    type SlotInfo,
  } from "../cameraLayout";

  // Every region in a row, colored by its worst problem. Clicking a row
  // selects that region.
  interface Props {
    doc: ConfigDocument;
    slots: SlotInfo[];
    selected: number;
    onselect: (cameraId: number) => void;
  }

  let { doc, slots, selected, onselect }: Props = $props();
</script>

<SettingsCard title="Assignments">
  <Table hoverable>
    <TableHead>
      <TableHeadCell class="px-2 py-1.5">ID</TableHeadCell>
      <TableHeadCell class="px-2 py-1.5">Region</TableHeadCell>
      <TableHeadCell class="px-2 py-1.5">Host</TableHeadCell>
      <TableHeadCell class="px-2 py-1.5">Device</TableHeadCell>
      <TableHeadCell class="px-2 py-1.5">Live</TableHeadCell>
      <TableHeadCell class="px-2 py-1.5">Config</TableHeadCell>
      <TableHeadCell class="px-2 py-1.5">Calibration</TableHeadCell>
      <TableHeadCell class="px-2 py-1.5">Warnings</TableHeadCell>
    </TableHead>
    <TableBody>
      {#each slots as s (s.cameraId)}
        {@const camera = s.camera}
        {@const rowStatus = cameraStatus(s.cameraId)}
        <TableBodyRow
          class={`cursor-pointer ${s.severity === "warning" ? "bg-yellow-50 dark:bg-yellow-900/30" : ""} ${s.severity === "error" ? "bg-red-50 dark:bg-red-900/30" : ""} ${s.cameraId === selected ? "font-semibold" : ""}`}
          onclick={() => {
            onselect(s.cameraId);
          }}
        >
          <TableBodyCell class="px-2 py-1.5">{s.cameraId}</TableBodyCell>
          <TableBodyCell class="px-2 py-1.5">{s.region}</TableBodyCell>
          {#if camera}
            <TableBodyCell class="px-2 py-1.5">
              <span class="flex items-center gap-1.5">
                <span
                  class={`inline-block h-2 w-2 rounded-full ${hostColor(doc, camera.instance ?? "")}`}
                ></span>
                {hostName(camera)}
              </span>
            </TableBodyCell>
            <TableBodyCell
              class="px-2 py-1.5"
              title={cameraDevice(doc, camera).path}
            >
              {deviceLabel(cameraDevice(doc, camera))}
            </TableBodyCell>
            <TableBodyCell
              class={`px-2 py-1.5 ${s.conflict ? "text-red-700" : ""}`}
            >
              {liveLabel(s.sources)}
            </TableBodyCell>
            <TableBodyCell class="px-2 py-1.5">
              {camera.configPath ? "local" : "remote"}
            </TableBodyCell>
            <TableBodyCell class="px-2 py-1.5">
              {rowStatus?.calibration ?? "none"}
            </TableBodyCell>
            <TableBodyCell
              class="px-2 py-1.5"
              title={(rowStatus?.warnings ?? [])
                .map((w) => w.message)
                .join("\n")}
            >
              {#if (rowStatus?.warnings.length ?? 0) > 0}
                <span class="text-yellow-700">
                  ⚠ {rowStatus?.warnings.map((w) => w.code).join(", ")}
                </span>
              {/if}
            </TableBodyCell>
          {:else}
            <TableBodyCell
              class="px-2 py-1.5 text-gray-500 dark:text-gray-400 italic"
            >
              No camera
            </TableBodyCell>
            <TableBodyCell></TableBodyCell>
            <TableBodyCell class="px-2 py-1.5"
              >{liveLabel(s.sources)}</TableBodyCell
            >
            <TableBodyCell class="px-2 py-1.5"></TableBodyCell>
            <TableBodyCell class="px-2 py-1.5"></TableBodyCell>
            <TableBodyCell class="px-2 py-1.5"></TableBodyCell>
          {/if}
        </TableBodyRow>
      {/each}
    </TableBody>
  </Table>
</SettingsCard>
