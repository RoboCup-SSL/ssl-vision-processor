# Reference configuration

Read-only reference files. Copy one to the repo root under your own name and
edit the copy; git ignores config files outside this directory, so local
configs never get committed by accident.

| File | What it is |
| --- | --- |
| `vision.yml` | The unified config the GUI host owns: field, shared and per-camera settings, locked calibrations. Start here. |
| `legacy/config.yml` | vision_processor's own config, with every setting and its default documented. The GUI generates one of these per local camera from `vision.yml`. |
| `legacy/config-minimal.yml` | The smallest vision_processor config: just what has no usable default. |
| `legacy/geometry-divA.yml` | Division A field (12 x 9 m). The GUI's Division A preset. |
| `legacy/geometry-divB.yml` | Division B field (9 x 6 m). The GUI's Division B preset. |

## Getting started

```sh
cp config/vision.yml vision.yml
./gui/bin/vision-processor-gui -config vision.yml
```

For a vision_processor run by hand, without the GUI:

```sh
cp config/legacy/config.yml config.yml
./build/vision_processor config.yml
```

## Read-only

These files are `chmod 444`, and the GUI refuses to write any file without
write permission, so a `config_path` or Save As pointed here fails instead of
overwriting a reference. Git doesn't record read-only permissions, so a fresh
clone needs:

```sh
chmod 444 config/*.yml config/legacy/*.yml
```

The GUI's tests parse every file here, so a reference that stops matching
what the code accepts fails `make test`.
