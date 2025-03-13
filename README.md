# herosync

**Download, combine, and publish GoPro videos with ease.**

---

A tool for automating GoPro video transfers. Download media files over WiFi,
combine chapters into complete videos, clean up storage, and optionally publish
to YouTube. Designed for hands-free operation via cron jobs or interactive use
with detailed status reporting.

## Usage

```
Download, combine, and publish GoPro videos with ease

Usage:
  herosync [command]

Available Commands:
  status      Display GoPro hardware and storage info
  list        Show media inventory and sync state details
  download    Fetch new media files from the GoPro
  combine     Merge original media into processed videos
  publish     Upload processed videos to YouTube
  cleanup     Delete transferred media from GoPro storage
  yolo        Hands-free sync: download, combine, publish
  help        Help about any command

Flags:
  -c, --config-file string    configuration file path
                              [env: HEROSYNC_CONFIG_FILE]
                              [default: ~/Library/Application Support/herosync/config.toml]

      --gopro-host string     GoPro URL host (IP, hostname:port, "" for mDNS discovery)
                              [env: HEROSYNC_GOPRO_HOST]
                              [default: ""]

      --gopro-scheme string   GoPro URL scheme (http, https)
                              [env: HEROSYNC_GOPRO_SCHEME]
                              [default: http]

  -h, --help                  help for herosync

  -l, --log-level string      logging level (debug, info, warn, error)
                              [env: HEROSYNC_LOG_LEVEL]
                              [default: info]

  -o, --output-dir string     output directory path
                              [env: HEROSYNC_OUTPUT_DIR]
                              [default: ~/Library/Application Support/herosync/media]

Use "herosync [command] --help" for more information about a command.
```

## Caveats

`herosync` was designed specifically for my niche use case and will likely **not
function out of the box** for more common workflows without additional
configuration.

For reference, my setup includes:

- Running the [GoPro Labs firmware](https://gopro.com/en/us/info/gopro-labs).
- Using a _GoPro HERO13_; older models may lack support for some
  [Labs extensions](https://gopro.github.io/labs/control/extensions/) that
  `herosync` depends on.
- Enabling the
  [Camera on Home Network](https://gopro.github.io/OpenGoPro/ble/features/cohn.html)
  (COHN) feature, eliminating the need for Bluetooth Low Energy (BLE) pairing
  before WiFi connection.
- Configuring the "Open Network" (`OPNW=1`) setting for faster HTTP access
  without requiring HTTPS or basic authentication.
- Not preserving [GPMF telemetry data](https://gopro.github.io/gpmf-parser/), as
  it’s not needed for my workflow.
- Keeping my GoPro **always on** and continuously powered via USB.

## License

herosync is released under the [Zero Clause BSD License][LICENSE] (SPDX: 0BSD).

Copyright &copy; 2025 [Aaron Bull Schaefer][EMAIL] and contributors

[LICENSE]: https://github.com/EarthmanMuons/herosync/blob/main/LICENSE
[EMAIL]: mailto:aaron@elasticdog.com
