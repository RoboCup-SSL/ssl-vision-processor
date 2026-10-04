<script lang="ts">
  import { Li, List } from "flowbite-svelte";
  import { config } from "../../config.svelte";
  import { wizard } from "../wizard.svelte";
  import { OPTIONAL_LINE_FIELDS } from "../../fieldConfigFields";

  let markings = $derived(
    OPTIONAL_LINE_FIELDS.filter((f) => wizard.draft.optionalFieldLines[f.key]),
  );
</script>

<div class="flex flex-col gap-3">
  <p class="text-sm text-gray-700">
    Review, then apply to put these values live on the field.
  </p>

  <List class="flex flex-col gap-1 text-sm text-gray-700">
    <Li>
      Field: {wizard.draft.field.fieldLength ?? 0}mm x {wizard.draft.field
        .fieldWidth ?? 0}mm
    </Li>
    <Li>
      Markings:
      {markings.length > 0 ? markings.map((f) => f.label).join(", ") : "none"}
    </Li>
  </List>

  <p class="text-sm text-gray-600">
    Applying takes effect immediately but doesn't save -- use Save in the
    settings menu (or Ctrl+S) to write <code
      >{config.state?.path ?? "vision.yml"}</code
    >, then calibrate each camera on its own Geometry tab.
  </p>
</div>
