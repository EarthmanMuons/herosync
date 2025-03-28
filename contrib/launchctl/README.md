# Automating `herosync` with `launchctl` on macOS

To automate running `herosync` daily on macOS, follow these steps:

## Step 1: Create the LaunchAgents Directory

Open a terminal and run the following command to create the `LaunchAgents`
directory (if it doesn't already exist):

```bash
mkdir -p ~/Library/LaunchAgents
```

## Step 2: Configure and Save the Launch Agent

Edit the `com.earthmanmuons.herosync.plist` file to match your environment
before saving it. Update the following fields:

1. **Environment Variables:** Ensure that the `PATH` includes directories for
   `herosync`, `ffmpeg`, and `ffprobe`.

2. **Log File Paths:** Replace `CHANGEME` with your actual home directory path
   in the `StandardOutPath` and `StandardErrorPath` keys.

Save the modified file to the `~/Library/LaunchAgents/` directory.

## Step 3: Bootstrap the Launch Agent

To bootstrap the launch agent, run the following command:

```bash
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.earthmanmuons.herosync.plist
```

## Step 4: Trigger Network Permission Dialog (TCC)

To trigger macOS's Transparency, Consent, and Control (TCC) dialog for local
network access, run the following command:

```bash
launchctl kickstart gui/$(id -u)/com.earthmanmuons.herosync
```

This command will immediately perform an initial run of `herosync yolo`, which
should trigger the network permission prompt. Click **Allow** when prompted to
grant `herosync` permission to access the local network.

Once granted, check the log file to verify that the expected output is present:

```bash
tail -f ~/Library/Logs/herosync.log
```

Allowing local network access will make the application visible under the
**System Settings → Privacy & Security → Local Network** pane.

## Step 5: Verify the Launch Agent

To ensure the launch agent is loaded, run:

```bash
launchctl list | grep herosync
```

If the service is listed, it’s successfully loaded. For more detailed diagnostic
information, use:

```bash
launchctl print gui/$(id -u)/com.earthmanmuons.herosync
```

If successful, the `herosync yolo` command will run automatically every night at
11 PM.

## References

- [Script management with launchd in Terminal on Mac](https://support.apple.com/guide/terminal/script-management-with-launchd-apdc6c1077b-5d5d-4d35-9c19-60f2397b2369/mac)
- [Understanding Local Network Privacy](https://developer.apple.com/documentation/technotes/tn3179-understanding-local-network-privacy)
- [How to reset (remove) apps from "Local Network" privacy settings?](https://developer.apple.com/forums/thread/766270)
