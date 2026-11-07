# Reference configuration

Read-only reference files for working in this repo. The GUI needs none of them
to run: it embeds the two field presets, and creates a `vision.yml` by itself
(see [gui/README.md](../gui/README.md)).

Copy one to the repo root under your own name and edit the copy; git ignores
config files outside this directory, so local configs never get committed by
accident.

| File | What it is |
| --- | --- |
| `vision.yml` | The unified config the GUI host owns: field, shared and per-camera settings, locked calibrations. Start here. |
| `legacy/config.yml` | vision_processor's own config, with every setting and its default documented. The GUI generates one of these per local camera from `vision.yml`. |
| `legacy/config-minimal.yml` | The smallest vision_processor config: just what has no usable default. |
| `legacy/geometry-divA.yml` | Division A field (12 x 9 m). Embedded as the GUI's Division A preset. |
| `legacy/geometry-divB.yml` | Division B field (9 x 6 m). Embedded as the GUI's Division B preset. |

## Getting started

```sh
cp config/vision.yml vision.yml
cd gui && make run
```

For a vision_processor run by hand, without the GUI:

```sh
cp config/legacy/config.yml config.yml
./build/vision_processor config.yml
```

## Read-only

These files are `chmod 444`. The GUI refuses to write any file without write
permission, and only writes in its config folder, so it won't overwrite one.
Git doesn't record read-only permissions, so a fresh
clone needs:

```sh
chmod 444 config/*.yml config/legacy/*.yml
```

The GUI's tests parse every file here, so a reference that stops parsing fails
`make test`. Only `vision.yml` and the geometry files are checked against what
the code accepts.
