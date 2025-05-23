# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

- `make build` - Build the capscreen binary to `build/capscreen`
- `make run` - Build and run the application
- `make install` - Install capscreen globally using `go install`
- `make deps` - Update Go module dependencies with `go mod tidy`
- `make clean` - Remove the build directory

## Architecture

This is a terminal-based screen recording application written in Go that captures video and audio simultaneously. The application uses:

- **Bubble Tea framework** for the interactive TUI with state machine pattern
- **FFmpeg** for video/audio recording and encoding
- **X11/xrandr** for display detection and capture
- **PulseAudio/pactl** for audio source management

### Core Components

- **State Machine**: Uses enum-based states (`selectDisplay`, `selectSource`, `recording`, `done`) to manage UI flow
- **Display Detection**: Parses `xrandr` output to get monitor geometry and offsets
- **Audio Sources**: Uses `pactl` to enumerate available PulseAudio sources
- **Recording Process**: Spawns FFmpeg subprocess with X11grab and PulseAudio inputs

### Key Dependencies

The application requires these system commands to be available:
- `xrandr` - X11 display configuration
- `pactl` - PulseAudio control
- `ffmpeg` - Video/audio recording

### Entry Point

The main function in `main.go:346` handles CLI argument parsing and launches the Bubble Tea program. Recording output files are timestamped MP4 files saved to the current directory.