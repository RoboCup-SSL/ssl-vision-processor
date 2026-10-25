# Camera drivers

Camera backends behind the `CameraDriver` interface ([cameradriver.h](cameradriver.h)), selected by
`camera.driver` in the config. For league camera hardware see the
[SSL docs: Cameras and Lenses](https://robocup-ssl.github.io/ssl-docs/field/cameras.html).

## Spinnaker (Teledyne FLIR)

| Camera | Interface | Sensor | Bayer | Docs |
|---|---|---|---|---|
| Blackfly S BFS-U3-51S5C-C (league) | USB3 | Sony IMX250, 2448×2048 | RG | [Product docs](https://softwareservices.flir.com/BFS-U3-51S5/latest/Model/Readme.html), [node reference](https://softwareservices.flir.com/BFS-U3-51S5/latest/Model/public/index.html) |
| Blackfly BFLY-PGE-13E4C-CS (first gen) | GigE PoE | e2v EV76C560, 1280×1024 | BG | [Technical reference v14](https://www.eureca.de/files/pdf/optoelectronics/flir/BFLY-PGE-Technical-Reference.pdf) (distributor mirror) |

**Generational differences.** The Blackfly S uses standard GenICam (SFNC) node names. The first
generation Blackfly predates them. With Spinnaker it mostly exposes the standard names too, sometimes
next to its old ones (e.g. both `AcquisitionFrameRateEnable` and `AcquisitionFrameRateEnabled`), but
not everywhere. On the first generation BFLY-PGE-13E4C (verified with Spinnaker 4.4; the league
Blackfly S has all of these):

- `GammaEnabled` instead of `GammaEnable`, and gamma is unavailable in Raw/Bayer pixel formats.
- No `AcquisitionResultingFrameRate`, `AutoExposureMeteringMode`, `AutoExposureControlPriority` or
  `BalanceWhiteAutoProfile`.

The 2017 technical reference predates Spinnaker and lists some names (e.g. `DefaultUserSet`,
`AcquisitionFrameRateControlEnabled`) that this firmware does not expose. Trust a probe dump over it.

The driver skips features a camera lacks (warning once) and derives the Bayer pattern from the camera
([spinnakerfeatures.h](spinnakerfeatures.h)). `build/spinnaker_probe` dumps a camera's nodes.

## mvIMPACT (Matrix Vision Bluefox3)

TODO

## OpenCV (V4L2 cameras, image and video files)

TODO

## Dummy

TODO
