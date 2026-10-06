// Which file dialog is open. Module-level so the gear menu (which unmounts its
// contents when it closes) can open a dialog that outlives it.
export const fileDialogs = $state({
  save: false,
  saveAs: false,
  load: false,
  // The external change the user answered "Later" to; it re-prompts once the
  // file changes again.
  dismissedExternal: "",
});
