<script lang="ts">
  import { Badge, Button, Modal, P, Spinner } from "flowbite-svelte";
  import type { GitState } from "../apiTypes";
  import RichText from "../RichText.svelte";
  import StatusAlert from "../StatusAlert.svelte";
  import { about as text } from "../text/about";
  import { hostSystem, loadHostSystem } from "./hostSystem.svelte";

  // The settings menu's About: what the host was built from, as the host
  // reports it (internal/buildinfo). The host embeds this page, so its build
  // is the page's too, except under Vite's dev server.
  interface Props {
    open?: boolean;
    onreport: () => void;
  }

  let { open = $bindable(false), onreport }: Props = $props();

  $effect(() => {
    if (open) void loadHostSystem();
  });

  let system = $derived(hostSystem.info);
  let error = $derived(hostSystem.error);
  let build = $derived(system?.build);
  let repos = $derived(
    build
      ? [
          { name: text.repo, git: build.repo },
          { name: text.proto, git: build.proto },
        ]
      : [],
  );
  let anyDirty = $derived(repos.some((r) => r.git.dirty));

  function badge(git: GitState): {
    label: string;
    color: "green" | "yellow" | "gray";
  } {
    if (!git.commit) return { label: text.unknown, color: "gray" };

    return git.dirty
      ? { label: text.dirty, color: "yellow" }
      : { label: text.clean, color: "green" };
  }
</script>

<Modal bind:open title={text.title} size="md" class="fixed inset-0 m-auto">
  {#if error}
    <StatusAlert color="red" class="p-2 text-sm">
      {text.hostUnavailable(error)}
    </StatusAlert>
  {:else if !build || !system}
    <P class="flex items-center gap-2 text-sm"><Spinner size="4" /> Loading…</P>
  {:else}
    <P class="text-sm">
      Version <span class="font-mono">{build.version}</span>
    </P>

    {#each repos as repo (repo.name)}
      {@const s = badge(repo.git)}
      <div>
        <div class="mb-1 flex items-center justify-between gap-2">
          <span class="text-sm font-semibold">{repo.name}</span>
          <Badge color={s.color}>{s.label}</Badge>
        </div>
        <dl
          class="grid grid-cols-[6rem_1fr] gap-x-3 gap-y-0.5 text-sm text-gray-600 dark:text-gray-400"
        >
          <dt>Branch</dt>
          <dd class="truncate font-mono">
            {repo.git.commit ? repo.git.branch || text.detached : "-"}
          </dd>
          <dt>Tag</dt>
          <dd class="truncate font-mono">
            {repo.git.commit ? repo.git.tag || text.noTag : "-"}
          </dd>
          <dt>Commit</dt>
          <dd class="truncate font-mono select-all" title={repo.git.commit}>
            {repo.git.commit || "-"}
          </dd>
        </dl>
      </div>
    {/each}

    {#if anyDirty}
      <StatusAlert color="yellow" class="p-2 text-sm">
        {text.dirtyNote}
      </StatusAlert>
    {/if}
    {#if !build.stamped}
      <P class="text-sm text-gray-600 dark:text-gray-400">
        <RichText text={text.unstamped} />
      </P>
    {/if}
    {#if import.meta.env.DEV}
      <P class="text-sm text-gray-600 dark:text-gray-400">
        <RichText text={text.devServer} />
      </P>
    {/if}

    <P class="text-xs text-gray-500 dark:text-gray-400">
      {build.goVersion} · {system.host.os}/{system.host.arch}
    </P>
  {/if}

  <div
    class="flex justify-end gap-2 border-t border-gray-200 pt-4 dark:border-gray-700"
  >
    <Button
      color="alternative"
      onclick={() => {
        open = false;
        onreport();
      }}>System report…</Button
    >
    <Button
      onclick={() => {
        open = false;
      }}>Close</Button
    >
  </div>
</Modal>
