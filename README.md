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
  combine     Merge incoming media into outgoing videos
  publish     Upload outgoing videos to YouTube
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

  -m, --media-dir string      parent directory for media storage
                              [env: HEROSYNC_MEDIA_DIR]
                              [default: ~/Library/Application Support/herosync/media]

Use "herosync [command] --help" for more information about a command.
```

### YouTube Authorization Credentials

In order to access YouTube programatically for publishing, you'll need to turn
on the _YouTube Data API_ for your Google account:

1. Use [this wizard](https://console.developers.google.com/start/api?id=youtube)
   to create or select a project in the Google Developers Console and
   automatically turn on the API. Click **Continue**, then **Go to
   credentials**.

2. On the **Create credentials** page, click the **Cancel** button.

3. At the top of the page, select the **OAuth consent screen** tab. Select an
   **Email address**, enter a **Product name** if not already set, and click the
   **Save** button.

4. Select the **Credentials** tab, click the **Create credentials** button and
   select **OAuth client ID**.

5. Select the application type **Other**, enter the name "herosync", and click
   the **Create** button.

6. Click **OK** to dismiss the resulting dialog.

7. Click the **(Download JSON)** button to the right of the client ID.

8. Move the downloaded file to your herosync config directory and rename it
   `client_secret.json`.

These steps were adapted from the upstream
[Go Quickstart](https://developers.google.com/youtube/v3/quickstart/go)
documentation.

### YouTube Category IDs

Every video requires a _Category ID_, which by default is `"22"` for **People &
Blogs**. Here are the assignable ID's for reference:

| Category              | ID  |
| --------------------- | --- |
| Autos & Vehicles      | 2   |
| Comedy                | 23  |
| Education             | 27  |
| Entertainment         | 24  |
| Film & Animation      | 1   |
| Gaming                | 20  |
| Howto & Style         | 26  |
| Music                 | 10  |
| News & Politics       | 25  |
| Nonprofits & Activism | 29  |
| People & Blogs        | 22  |
| Pets & Animals        | 15  |
| Science & Technology  | 28  |
| Sports                | 17  |
| Travel & Events       | 19  |

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
