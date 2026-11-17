<script lang="ts">
  import { Alert, type AlertProps } from "flowbite-svelte";

  // Flowbite's Alert, with readable dark-mode colors for red and yellow.
  // flowbite-svelte turns those into bright pastel boxes in dark mode
  // (dark:bg-*-200), unlike its primary Alert and Flowbite's own design. Its
  // class merging lets these dark classes replace that, so this doesn't
  // depend on its internal class names. Use it instead of Alert.
  const DARK: Partial<Record<string, string>> = {
    yellow: "dark:bg-gray-800 dark:text-yellow-300",
    red: "dark:bg-gray-800 dark:text-red-400",
  };

  let {
    color = "primary",
    class: className = "",
    children,
    ...rest
  }: AlertProps = $props();
</script>

<Alert {color} class={[DARK[color], className]} {...rest}
  >{@render children()}</Alert
>
