<script lang="ts">
  import RichText from "../../RichText.svelte";
  import { Li, List } from "flowbite-svelte";
  import { config } from "../../config.svelte";
  import { wizard, wizardCameraCount } from "../wizard.svelte";
  import { cameraCount } from "../../cameraLayout";
  import { OPTIONAL_LINE_FIELDS } from "../../fieldConfigFields";
  import { wizard as text } from "../../text/wizard";

  let count = $derived(wizardCameraCount());
  let changes = $derived(
    config.doc ? count !== cameraCount(config.doc) : false,
  );

  let markings = $derived(
    OPTIONAL_LINE_FIELDS.filter((f) => wizard.draft.optionalFieldLines[f.key]),
  );
</script>

<div class="flex flex-col gap-3">
  <p class="text-sm text-gray-700 dark:text-gray-300">
    {text.review}
  </p>

  <List class="flex flex-col gap-1 text-sm text-gray-700 dark:text-gray-300">
    <Li>
      Field: {wizard.draft.field.fieldLength ?? 0}mm x {wizard.draft.field
        .fieldWidth ?? 0}mm
    </Li>
    <Li>
      Markings:
      {markings.length > 0 ? markings.map((f) => f.label).join(", ") : "none"}
    </Li>
    <Li>
      {text.finishCameras(count, changes)}
    </Li>
  </List>

  <p class="text-sm text-gray-600 dark:text-gray-400">
    <RichText text={text.applyNote(config.state?.path ?? "vision.yml")} />
  </p>
</div>
