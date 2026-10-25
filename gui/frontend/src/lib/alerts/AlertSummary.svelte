<script lang="ts">
  import { Badge, Heading, SidebarGroup, SidebarItem } from "flowbite-svelte";
  import { categoryHref } from "../layout/nav.svelte";
  import { alerts, SEVERITIES, type Severity } from "./alerts";

  let counts = $derived.by(() => {
    const all = alerts();
    const by = { error: 0, warning: 0, caution: 0 };
    for (const a of all) by[a.severity]++;

    return by;
  });

  // Text and badge colors when there's at least one; gray when there are
  // none, so a quiet system reads as quiet.
  const ACTIVE: Record<
    Severity,
    { text: string; badge: "red" | "orange" | "yellow" }
  > = {
    error: { text: "text-red-700", badge: "red" },
    warning: { text: "text-orange-600", badge: "orange" },
    caution: { text: "text-yellow-600", badge: "yellow" },
  };

  // The link opens the Alerts page; this then brings that section into view
  // once the page has rendered.
  function scrollTo(id: Severity): void {
    setTimeout(() => {
      document
        .getElementById(`alerts-${id}`)
        ?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 0);
  }
</script>

<Heading
  tag="h2"
  class="px-2 text-xs font-semibold tracking-wider text-gray-500 dark:text-gray-400 uppercase"
>
  Alerts
</Heading>

<SidebarGroup class="mt-0.5 mb-3 space-y-0">
  {#each SEVERITIES as { id, label } (id)}
    {@const count = counts[id]}
    <SidebarItem
      href={categoryHref("alerts")}
      {label}
      spanClass={`flex-1 ${count > 0 ? `font-semibold ${ACTIVE[id].text}` : "text-gray-400"}`}
      aClass="py-0.5!"
      onclick={() => {
        scrollTo(id);
      }}
    >
      {#snippet subtext()}
        <Badge
          rounded
          color={count > 0 ? ACTIVE[id].badge : "gray"}
          class={`px-1.5 py-0 text-xs ${count > 0 ? "" : "opacity-60"}`}
          aria-label={`${String(count)} ${label.toLowerCase()}`}
        >
          {count}
        </Badge>
      {/snippet}
    </SidebarItem>
  {/each}
</SidebarGroup>
