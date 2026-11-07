# vision_processor GUI

A browser interface for setting up and running [vision_processor](../README.md), the SSL's shared vision system.
It holds your field geometry and camera settings, locks in camera calibrations, shows live video and the live
field, and publishes the field geometry to the rest of the league's software.

It is one self-contained program: download it and run it. It needs no installation, no root, and no other files.

Working on the GUI itself? See [DEVELOPMENT.md](DEVELOPMENT.md).

## Quick start

These steps are for 64-bit Linux, tested on Ubuntu 24.04.

1. Download the `.tar.gz` for your machine from the
   [releases page](https://github.com/RoboCup-SSL/ssl-vision-processor/releases), along with `SHA256SUMS`. Then
   check the download and unpack it:

   ```sh
   sha256sum --check --ignore-missing SHA256SUMS
   tar -xzf ssl-vision-processor-gui-*-linux-amd64.tar.gz
   cd ssl-vision-processor-gui-*-linux-amd64
   ```

2. Run it:

   ```sh
   ./ssl-vision-processor-gui
   ```

   It prints where it keeps everything, then the address to open:

   ```
   INF paths configDir=/home/you/.config/rcssl/vision-processor-gui configFile=vision.yml
   INF paths logFile=/home/you/.local/state/rcssl/vision-processor-gui/logs/vision-processor-gui.log
   INF paths imgDir=/home/you/.local/state/rcssl/vision-processor-gui/img
   INF UI is available url=http://192.168.1.20:8085
   ```

3. Open that address in a browser. The setup wizard opens by itself: choose your division, or enter your field's
   dimensions if you have a non-standard field size.

4. Save when the wizard asks. **The GUI publishes nothing on the network until this first save**, so you can try
   it on a live network without disturbing anyone. Until then, an alert reminds you that it isn't publishing.

The program stops when you close the terminal or press Ctrl+C, and picks up where it left off the next time you
run it. It doesn't start by itself when the machine boots; packages with a system service are planned.

## Where things are

| What                   | Default location                                                                     | Change it with |
| ---------------------- | ------------------------------------------------------------------------------------ | -------------- |
| Config folder          | `~/.config/rcssl/vision-processor-gui/`                                              | `-configDir`   |
| Your configuration     | `vision.yml` in the config folder                                                    | `-configFile`  |
| Robot height table     | `robot-heights.yml` in the config folder                                             | `vision.yml`   |
| Log file               | `~/.local/state/rcssl/vision-processor-gui/logs/vision-processor-gui.log`, rotated   | `-logFile`     |
| Debug snapshot images  | `~/.local/state/rcssl/vision-processor-gui/img/`                                     | `-imgDir`      |

`XDG_CONFIG_HOME` and `XDG_STATE_HOME` replace `~/.config` and `~/.local/state` if you have them set.

**Back up the config folder.** `vision.yml` holds your field, camera settings, and locked calibrations.

To keep a whole setup together, for example on a USB stick, point everything at one place:

```sh
./ssl-vision-processor-gui -configDir ./setup -logFile ./setup/logs/gui.log -imgDir ./setup/img
```

To start over, stop the program and delete or rename `vision.yml`. The next run creates a new one and starts the
wizard again.

### How configuration is saved

Edits in the browser apply immediately, but are written to `vision.yml` only when you save. Saving lists every
change first. If `vision.yml` changes on disk while the GUI is running, it asks whether to load that version or
overwrite it. Saving rewrites the file, so comments you add by hand are lost.

Every config file lives directly in the config folder, with no subfolders. Save As and Load take a file name, not
a path, and `config_path` and `bot_heights_file` in `vision.yml` must also be plain file names. A `vision.yml` that
names a full path won't load until you change it.

## Options

Every option is optional. `-h` lists them all.

| Option         | Default              | What it does                                                                       |
| -------------- | -------------------- | ---------------------------------------------------------------------------------- |
| `-address`     | `:8085`              | Address and port for the GUI. `127.0.0.1:8085` allows this machine only.           |
| `-configDir`   | see above            | The config folder. Created if missing.                                             |
| `-configFile`  | `vision.yml`         | The file to use in the config folder: a file name, not a path. Created if missing. |
| `-logFile`     | see above            | Rotating log file. `-logFile ""` turns it off.                                     |
| `-imgDir`      | see above            | Where the GUI looks for vision_processor's debug images.                           |
| `-logLevel`    | `Info`               | `Debug`, `Info`, `Warn`, or `Error`.                                               |
| `-allowOrigin` | none                 | Other web pages allowed to use the API on a competition field.                     |
| `-allowHost`   | none                 | Extra names this machine is reached by, such as a DNS alias.                       |
| `-version`     |                      | Print the version and exit.                                                        |
| `-readme`      |                      | Print this guide and exit, e.g. `./ssl-vision-processor-gui -readme \| less`.      |

## Network setup

Multicast addresses are set on the GUI's Network page; these are the defaults.

| Traffic                 | Address                                         | Direction  |
| ----------------------- | ----------------------------------------------- | ---------- |
| GUI in a browser        | TCP `8085`                                      | In         |
| Vision (geometry, detections) | UDP `224.5.23.2:10006`                    | In and out |
| Game controller         | UDP `224.5.23.1:10003`                          | In         |
| Autoref tracker         | UDP `224.5.23.2:10010`                          | In         |
| Live video, per camera  | UDP `224.5.23.100:10100` for camera 0, `.101` for camera 1, and so on | In |

The GUI uses every network interface that is up, has a cable or link, supports multicast, and has an IPv4 address.
It skips loopback, Docker and VM bridges, and VPNs. The Network page lets you exclude interfaces.

Three settings can trip you up on a stock Ubuntu machine. All need root, so the GUI can't change them for you.

**Firewall.** Ubuntu's `ufw` is usually off, but if `sudo ufw status` says it's active, allow the GUI's traffic:

```sh
sudo ufw allow 8085/tcp
sudo ufw allow 10003,10006,10010,10100/udp
```

**Live video stutters or breaks up.** Linux's default network receive buffer is too small for a video keyframe,
and the GUI gets no warning when it's capped. Raise the limit and keep it across reboots:

```sh
echo 'net.core.rmem_max=4194304' | sudo tee /etc/sysctl.d/60-rcssl-multicast.conf
sudo sysctl --system
```

**Nothing is heard with vision_processor on the same machine and no network cable.** Traffic then has to go over
loopback, where Ubuntu turns multicast off. The GUI logs this at startup. Turn it on, which lasts until the next
reboot:

```sh
sudo ip link set lo multicast on
```

### Browser

Live video needs H.264 support. Google Chrome includes it. Chromium and Firefox on Linux rely on the system's
FFmpeg libraries, so if video stays blank there, try Chrome first.

## Security

⚠️ **The GUI has no login.** Anyone who can reach its port can change the running configuration and overwrite
files in the config folder, including locked calibrations. It can't write outside that folder through the API.

Run it only on a network you trust, or use `-address 127.0.0.1:8085` to allow this machine only. Marking a setup
as a competition field limits which web pages can use the API, but that only applies to browsers.

## Cameras

This download is the GUI only. It doesn't include vision_processor, which does the camera processing. For now you
build that from source: see the [main README](../README.md). The GUI finds running vision_processors through the
multicast traffic they send, wherever they run.

Camera settings changed in the GUI reach a vision_processor through a generated config file, so that only works
for one on the same machine. To use it, add `config_path: <file name>` to the camera in `vision.yml`, then start
vision_processor with that file from the config folder. Delivering settings over the network is planned.

Debug snapshots also need vision_processor on the same machine, with `-imgDir` pointing at its `img/` folder.

## Troubleshooting

Check the GUI's Alerts page first; most alerts have a how-to. The printed log file has the details.

| Problem                                      | Likely cause and fix                                                                            |
| -------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `http server init failed` and it exits       | Something else is using the port, often another copy of the GUI. Stop it, or use another `-address`. |
| `no default config folder, set -configDir`   | `HOME` isn't set, as in some service environments. Set `-configDir`, `-logFile`, and `-imgDir`. |
| Page doesn't load from another machine       | Firewall; see [Network setup](#network-setup). Check the address the GUI printed.               |
| "This host doesn't answer to..."             | You're using a name the GUI doesn't know. Add it with `-allowHost`.                             |
| Nothing is published                         | First run: save once. Then check the Alerts page.                                               |
| Your old settings seem to be gone            | The GUI no longer reads `vision.yml` from the current folder. Pass `-configDir .` from that folder. |
| `config_path: ... want a file name`          | `vision.yml` names a full path. Move the file to the config folder and use only its name.       |

## Upgrading

Stop the program and replace the binary. Your configuration is kept in the config folder. `-version` shows which
version you have.

## License

Apache 2.0; see `LICENSE.md`. The release's `THIRD_PARTY_NOTICES.txt` lists the licenses of everything bundled.
