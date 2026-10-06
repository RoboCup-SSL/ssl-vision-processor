# User-facing text

Tooltips, warnings, error messages, and help text for the GUI live here, one
file per page or area. To change what the GUI says, edit these files; you
don't need to touch the components that show them.

| File              | What's in it                                              |
| ----------------- | --------------------------------------------------------- |
| `camera.ts`       | Camera Settings page                                      |
| `layout.ts`       | Camera Layout page and its confirm dialog                 |
| `network.ts`      | Network page, address and port advice, socket status      |
| `geometry.ts`     | Geometry page's calibration card                          |
| `alerts.ts`       | Alerts page titles and details                            |
| `docs.ts`         | The Alerts page's "How to fix" resolution docs            |
| `video.ts`        | Messages over live video                                  |
| `app.ts`          | Header badges, settings menu prompts, edit conflicts      |
| `wizard.ts`       | Setup wizard                                              |
| `color.ts`        | Color page, color picker, and update weights              |
| `placeholder.ts`  | Unbuilt pages and the Stream page                         |
| `configFields.ts` | Each config.yml setting's comment, copied from config.yml |
| `links.ts`        | Every external URL, by name                               |
| `common.ts`       | Phrases several pages share                               |

## Editing

- Plain text is a quoted string: change the words between the quotes.
- Text with values filled in is a small function, like
  ``(address: string): string => `Waiting for video on ${address}.` ``.
  Change the words around `${...}`; keep the `${...}` parts, which are the
  values.
- A list in square brackets is several lines shown together, such as the
  lines of one tooltip.
- `notes` and lists named after a setting are tooltips, shown with "Show
  extra tooltips" on. `warnings` and `errors` always show.
- Where an entry says `` `backticks` show as code ``, text between backticks
  shows as code, like `config_path`. Inside a template (text between
  backticks with `${...}` values), write a code backtick as `` \` ``.
- Fix a dead link in `links.ts`; components refer to links by name.

Keep the style the rest of the GUI uses: short, full sentences, no em or en
dashes.

## Adding text

New user-facing sentences go here, not in a component. Short labels, such as
button text and field names, can stay in the component. `npm run check:text`
(part of `make test`) fails on sentence-like strings written elsewhere. For a
sentence that is really code, such as an internal error a user never sees,
put a `// text-ok` comment (`<!-- text-ok -->` in markup) on the line above.

Translation (i18n) is planned for v2.1. These files' keys are meant to become
its message catalog then.
