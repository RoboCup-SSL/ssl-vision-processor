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
generation Blackfly predates them, so some nodes differ:

- Renamed: `GammaEnabled`, `DefaultUserSet`, `AcquisitionFrameRateControlEnabled` (per the manual,
  unconfirmed on hardware).
- Missing: `AcquisitionResultingFrameRate`, `AutoExposureMeteringMode`, `AutoExposureControlPriority`,
  `BalanceWhiteAutoProfile`.
- Gamma is unavailable in Raw/Bayer pixel formats.

The driver resolves these at runtime from the candidate names in
[spinnakerfeatures.h](spinnakerfeatures.h) and derives the Bayer pattern from the camera.
`build/spinnaker_probe` dumps a camera's nodes and shows what the driver resolves on it.

## mvIMPACT (Matrix Vision Bluefox3)

TODO

## OpenCV (V4L2 cameras, image and video files)

TODO

## Dummy

TODO
