<script lang="ts">
  import { virtualField } from "../../geometry.svelte";
  import { wizard } from "../wizard.svelte";
  import { OPTIONAL_LINE_FIELDS } from "../../fieldConfigFields";

  let markings = $derived(
    OPTIONAL_LINE_FIELDS.filter((f) => wizard.draft.optionalFieldLines[f.key]),
  );
</script>

<div class="flex flex-col gap-3">
  <p class="text-sm text-gray-700">
    Review, then apply to load these values into the editor.
  </p>

  <ul class="flex flex-col gap-1 text-sm text-gray-700">
    <li>
      Field: {wizard.draft.field.fieldLength ?? 0}mm x {wizard.draft.field
        .fieldWidth ?? 0}mm
    </li>
    <li>
      Markings:
      {markings.length > 0 ? markings.map((f) => f.label).join(", ") : "none"}
    </li>
  </ul>

  <p class="text-sm text-gray-600">
    Applying loads these values into the Virtual Field editor but doesn't save
    them -- use its own Save button to write <code
      >{virtualField.path || "geometry.yml"}</code
    >, then calibrate each camera on its own Geometry tab.
  </p>
</div>
