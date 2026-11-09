# vision_processor GUI

A browser interface for setting up and running [vision_processor](../README.md), the SSL's shared vision system.
It holds your field geometry and camera settings, locks in camera calibrations, shows live video and the live
field, and publishes the field geometry to the rest of the league's software.

It is one self-contained program: download it and run it. It needs no installation, no root, and no other files.
On Ubuntu you can [install the package](#installing-the-package) instead, which can also run it as a service.

Working on the GUI itself? See [DEVELOPMENT.md](DEVELOPMENT.md).

## Quick start

These steps are for 64-bit Linux, tested on Ubuntu 24.04.

1. Download the `.tar.gz` for your machine from the
   [releases page](https://github.com/RoboCup-SSL/ssl-vision-processor/releases), along with `SHA256SUMS`. Then
   check the download and unpack it:

   ```sh
   sha256sum --check --ignore-missing SHA256SUMS
   tar -xzf rcssl-vision-processor-gui-*-linux-amd64.tar.gz
   cd rcssl-vision-processor-gui-*-linux-amd64
   ```

2. Run it:

   ```sh
   ./rcssl-vision-processor-gui
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
run it. To have it start with the machine, [install the package](#installing-the-package).

## Installing the package

The releases page also has a `.deb` for Ubuntu 24.04, for `amd64` and `arm64` machines. It installs the same
program, and can run it as a system service that starts at boot.

```sh
sha256sum --check --ignore-missing SHA256SUMS
sudo apt install ./rcssl-vision-processor-gui_*_amd64.deb
```

It asks two questions:

1. **Run it as a system service?** Not as a service, start it at boot, or start it at boot and now. A
   non-interactive install, such as from the App Center or a script, takes the first answer: no service.
2. **Who can open it?** This machine only, or any machine on the network. Asked only for a service. The GUI has no
   login; see [Security](#security).

To change your answers: `sudo dpkg-reconfigure rcssl-vision-processor-gui`.

The service runs as its own user, `rcssl-vp-gui`, in the `rcssl` group that every RoboCup SSL tool's package
shares:

| What                   | Where                                                                          |
| ---------------------- | ------------------------------------------------------------------------------ |
| Config folder          | `/var/lib/rcssl/vision-processor-gui/`. Members of `rcssl` can read it.        |
| Log                    | `journalctl -u rcssl-vision-processor-gui`                                     |
| Address, extra options | `/etc/default/rcssl-vision-processor-gui`. Restart the service after editing.  |
| Start, stop, restart   | `sudo systemctl start rcssl-vision-processor-gui`, and so on                   |

To read the config folder without `sudo`, join the group (`sudo usermod -aG rcssl $USER`), then log in again.

### Customizing the service

Extra options go on the `ARGS` line of `/etc/default/rcssl-vision-processor-gui`, as you'd type them, for example
`ARGS="-logLevel Debug -allowHost vision.local"`. Then `sudo systemctl restart rcssl-vision-processor-gui`.
Reconfiguring the package changes only `ADDRESS` and keeps everything else in that file.

Don't edit the unit file itself, `/usr/lib/systemd/system/rcssl-vision-processor-gui.service`: upgrades replace it.
For anything `ARGS` can't do, add a drop-in, which upgrades keep:

```sh
sudo systemctl edit rcssl-vision-processor-gui     # writes /etc/systemd/system/rcssl-vision-processor-gui.service.d/override.conf
systemctl cat rcssl-vision-processor-gui           # shows the unit with every drop-in
```

The service is sandboxed: it can write only to its config folder. To have it write anywhere else, for example
`-imgDir` on another disk, add `ReadWritePaths=` with that folder in the drop-in. To replace the command entirely,
clear it first with an empty `ExecStart=` line, then give the new one.

Without the system service, run `rcssl-vision-processor-gui` as above, or as your own user service, which keeps the
same files as running it by hand:

```sh
systemctl --user enable --now rcssl-vision-processor-gui
```

The package also raises the network receive buffer limit that [live video needs](#network-setup).

Removing the package (`sudo apt remove`) keeps your settings, and purging it (`sudo apt purge`) keeps the config
folder too, since it holds your locked calibrations. Delete `/var/lib/rcssl/vision-processor-gui` yourself if you
want it gone.

The package doesn't include vision_processor; see [Cameras](#cameras). The service never runs cameras itself: on a
machine set up with packages, each vision_processor will run as its own service.

## Where things are

| What                   | Default location                                                                     | Change it with |
| ---------------------- | ------------------------------------------------------------------------------------ | -------------- |
| Config folder          | `~/.config/rcssl/vision-processor-gui/`                                              | `-configDir`   |
| Your configuration     | `vision.yml` in the config folder                                                    | `-configFile`  |
| Robot height table     | `robot-heights.yml` in the config folder                                             | `vision.yml`   |
| Log file               | `~/.local/state/rcssl/vision-processor-gui/logs/vision-processor-gui.log`, rotated   | `-logFile`     |
| Debug snapshot images  | `~/.local/state/rcssl/vision-processor-gui/img/`                                     | `-imgDir`      |
| Camera working folders | `~/.local/state/rcssl/vision-processor-gui/run/`, with `-runCameras` only           |                |

`XDG_CONFIG_HOME` and `XDG_STATE_HOME` replace `~/.config` and `~/.local/state` if you have them set.

**Back up the config folder.** `vision.yml` holds your field, camera settings, and locked calibrations.

To keep a whole setup together, for example on a USB stick, point everything at one place:

```sh
./rcssl-vision-processor-gui -configDir ./setup -logFile ./setup/logs/gui.log -imgDir ./setup/img
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
| `-runCameras`  | off                  | Run every camera's vision_processor on this machine; see [Cameras](#cameras).      |
| `-logLevel`    | `Info`               | `Debug`, `Info`, `Warn`, or `Error`.                                               |
| `-allowOrigin` | none                 | Other web pages allowed to use the API on a competition field.                     |
| `-allowHost`   | none                 | Extra names this machine is reached by, such as a DNS alias.                       |
| `-version`     |                      | Print the version and exit.                                                        |
| `-readme`      |                      | Print this guide and exit, e.g. `./rcssl-vision-processor-gui -readme \| less`.      |

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

**Firewall.** Ubuntu's `ufw` firewall is off on a new install, so most machines need nothing here. If you've turned
it on (`sudo ufw status` says `active`), it blocks the GUI's port and the traffic the GUI listens for until you
allow them. The GUI warns at startup when `ufw` is on. Allow its traffic with:

```sh
sudo ufw allow 8085/tcp
sudo ufw allow 10003,10006,10010,10100/udp
```

Use your own port in place of `8085` if you've changed it with `-address`.

**Live video stutters or breaks up.** Linux's default network receive buffer is too small for a video keyframe,
and the GUI gets no warning when it's capped. [The package](#installing-the-package) raises the limit for you.
Without it, raise the limit and keep it across reboots:

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

There are two ways to run the cameras:

- **On this machine, run by the GUI** (`-runCameras`). Simplest for one or two cameras, such as a lab webcam. The
  GUI starts a vision_processor for every camera, and restarts it when you change a setting it only reads at
  startup. Camera settings reach it through a file the GUI writes in the config folder, `cam0.yml` for camera 0
  and so on.
- **As their own services**, the default. For events and larger labs. The GUI starts nothing; each vision_processor
  is started separately, on this machine or others. Packages that set these up are planned.

### Running a webcam with the GUI

1. Build vision_processor, then make sure the shell finds it:

   ```sh
   export PATH=/path/to/vision-processor/build:$PATH
   which vision_processor
   ```

   The GUI uses the `vision_processor` on your `PATH`. There is no option to point it elsewhere.

2. Start the GUI with `-runCameras`:

   ```sh
   ./rcssl-vision-processor-gui -runCameras
   ```

   It prints which vision_processor it runs, and exits straight away if it can't find one. A new configuration's
   camera 0 is ready to go, on the first webcam (`/dev/video0`) once its driver is set to OPENCV on the Camera
   Settings page.

3. Camera Settings shows each vision_processor's state, its last lines of output, and a Restart button. Each camera
   you add gets its own config file automatically.

Each camera needs its own capture device. If two cameras name the same one, only the lower camera ID starts, and
an alert says which. The GUI doesn't know about a vision_processor started some other way: if one already has the
camera open, the GUI's copy says so in its output.

Each vision_processor runs in its own folder, `~/.local/state/rcssl/vision-processor-gui/run/cam<id>`, so its debug
files stay apart. Its snapshots go to the image folder.

## Troubleshooting

Check the GUI's Alerts page first; most alerts have a how-to. The printed log file has the details.

| Problem                                      | Likely cause and fix                                                                            |
| -------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `http server init failed` and it exits       | Something else is using the port, often another copy of the GUI. Stop it, or use another `-address`. |
| `no default config folder, set -configDir`   | `HOME` isn't set, as in some service environments. Set `-configDir`, `-logFile`, and `-imgDir`. |
| Page doesn't load from another machine       | It allows this machine only: check the address it printed, or the service's [setting](#installing-the-package). Then the firewall, if it's on; see [Network setup](#network-setup). |
| "This host doesn't answer to..."             | You're using a name the GUI doesn't know. Add it with `-allowHost`.                             |
| Nothing is published                         | First run: save once. Then check the Alerts page.                                               |
| Your old settings seem to be gone            | The GUI no longer reads `vision.yml` from the current folder. Pass `-configDir .` from that folder. |
| `config_path: ... want a file name`          | `vision.yml` names a full path. Move the file to the config folder and use only its name.       |
| `-runCameras needs vision_processor on PATH` | Build vision_processor and add its folder to `PATH`; see [Cameras](#cameras).                   |

## Upgrading

Stop the program and replace the binary. Your configuration is kept in the config folder. `-version` shows which
version you have.

With the package, install the newer `.deb` the same way. A running service restarts on the new version, and your
answers, the service's address, and whether it starts at boot are kept as they are.

## License

Apache 2.0; see `LICENSE.md`. The release's `THIRD_PARTY_NOTICES.txt` lists the licenses of everything bundled,
including the SSL protocol definitions. The package installs it and the license summary in
`/usr/share/doc/rcssl-vision-processor-gui/`, and a man page: `man rcssl-vision-processor-gui`.
