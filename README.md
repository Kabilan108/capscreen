# capscreen

a terminal-based screen recording tool for x11 linux systems that captures video and audio simultaneously.
basically a convenient wrapper around [ffmpeg](https://ffmpeg.org/).

## requirements

- x11 display server
- ffmpeg
- pulseaudio (`pactl` command)
- xrandr (x11 display utilities)

## installation

### using nix

```bash
# run directly without installing
nix run github:kabilan108/capscreen

# install to profile
nix profile install github:kabilan108/capscreen
```

### traditional linux install

1. **install dependencies** (ubuntu/debian):
   ```bash
   sudo apt install ffmpeg pulseaudio-utils x11-utils
   ```

2. **build from source**:
   ```bash
   git clone https://github.com/kabilan108/capscreen
   cd capscreen
   make build
   sudo cp build/capscreen /usr/local/bin/
   ```

## usage

### list available displays and audio sources:
```bash
capscreen list-displays
capscreen list-sources
```

### start recording:
```bash
# record primary display (no audio)
capscreen record

# record specific display with audio
capscreen record -d HDMI-1 -s alsa_output.pci-0000_00_1f.3.analog-stereo.monitor

# custom output directory and quality
capscreen record -o ~/recordings --quality 23 --fr 60
```

### recording options:
- `-d, --display <display>` - display to record (default: first display)
- `-s, --source <source>` - audio source (optional)
- `-o, --outdir <dir>` - output directory (default: current directory)
- `--fr <fps>` - frame rate (default: 30)
- `--quality <crf>` - video quality 0-51, lower = higher quality (default: 18)
- `--bitrate <rate>` - audio bitrate (default: 192k)

### stop recording:
- press `ctrl+c` or type `q` and press enter
- output files are saved as timestamped mp4 files (e.g., `2024.01.15_14.30.45.mp4`).
