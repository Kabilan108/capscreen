package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
)

type (
	Display struct{ Name, Resolution, OffsetX, OffsetY string }
	Source  struct{ Name, Status string }
)

var (
	errorColor = color.New(color.FgRed, color.Bold)
	flagColor  = color.New(color.FgGreen)
	infoColor  = color.New(color.FgBlue)
	warnColor  = color.New(color.FgYellow)
)

type Findable interface {
	Index() string
	String() string
}

type RecordOpts struct {
	xdisplay  string
	outpath   string
	display   *Display
	source    *Source
	framerate string
	quality   string
	bitrate   string
}

type Recording struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr bytes.Buffer
}

func (d Display) Index() string { return d.Name }
func (a Source) Index() string  { return a.Name }

func (d Display) String() string { return fmt.Sprintf("%s\t%s", d.Name, d.Resolution) }
func (a Source) String() string  { return fmt.Sprintf("%s\t%s", a.Name, a.Status) }

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func run(name string, args ...string) ([]string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(&out)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, nil
}

func checkExecutables() bool {
	for _, cmd := range []string{"xrandr", "pactl", "ffmpeg"} {
		if !commandExists(cmd) {
			return false
		}
	}
	return true
}

func validateDir(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("directory %s does not exist", path)
	} else if err != nil {
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", path)
	}
	return nil
}

func newRecordingFile(parent string) (string, error) {
	fn := fmt.Sprintf("%s.mp4", time.Now().Format("2006.01.02_03.04.05"))
	if err := validateDir(parent); err != nil {
		return "", err
	}
	return filepath.Join(parent, fn), nil
}

func findItem[T Findable](items []T, name string) *T {
	for _, i := range items {
		if i.Index() == name {
			return &i
		}
	}
	return nil
}

func listDisplays() ([]Display, error) {
	monitorLines, err := run("xrandr", "--listmonitors")
	if err != nil {
		return nil, err
	}
	geomLines, err := run("xrandr", "--query")
	if err != nil {
		return nil, err
	}
	ptn := regexp.MustCompile(`\d+x\d+\+\d+\+\d+`)

	var displays []Display
	for _, ml := range monitorLines[1:] {
		fields := strings.Fields(ml)
		if len(fields) != 4 {
			continue
		}
		name := fields[3]

		for _, gl := range geomLines {
			if strings.HasPrefix(gl, name) {
				m := ptn.FindString(gl)
				parts := strings.Split(m, "+")
				displays = append(displays, Display{
					Name:       name,
					Resolution: parts[0],
					OffsetX:    parts[1],
					OffsetY:    parts[2],
				})
				break
			}
		}
	}
	if len(displays) == 0 {
		return nil, fmt.Errorf("no displays found")
	}
	return displays, nil
}

func listSources() ([]Source, error) {
	lines, err := run("pactl", "list", "sources", "short")
	if err != nil {
		return nil, err
	}
	var sources []Source
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 7 {
			continue
		}
		sources = append(sources, Source{Name: fields[1], Status: fields[6]})
	}
	return sources, nil
}

func updateTimer(start time.Time, done <-chan bool) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			duration := time.Since(start)
			minutes := int(duration.Minutes())
			seconds := int(duration.Seconds()) % 60
			fmt.Fprint(os.Stderr, "\r", infoColor.Sprintf("recording: "), fmt.Sprintf("%02d:%02d", minutes, seconds))
		}
	}
}

func recordScreen(opts RecordOpts) (*Recording, error) {
	args := []string{
		"-f", "x11grab",
		"-r", opts.framerate,
		"-video_size", opts.display.Resolution,
		"-i", fmt.Sprintf("%s.0+%s,%s", opts.xdisplay, opts.display.OffsetX, opts.display.OffsetY),
	}
	if opts.source != nil {
		args = append(args, "-f", "pulse", "-i", opts.source.Name)
	}
	args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-crf", opts.quality, "-pix_fmt", "yuv420p")
	if opts.source != nil {
		args = append(args, "-c:a", "aac", "-b:a", opts.bitrate)
	}

	args = append(args, opts.outpath)

	cmd := exec.Command("ffmpeg", args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &Recording{cmd, stdin, stderr}, nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, infoColor.Sprint("usage:"), "capscreen <command> [-h|--help]")
	fmt.Fprintln(os.Stderr, infoColor.Sprint("commands:"))
	fmt.Fprintln(os.Stderr, "  "+flagColor.Sprint("list-displays")+"  list available displays")
	fmt.Fprintln(os.Stderr, "  "+flagColor.Sprint("list-sources")+"   list available audio sources")
	fmt.Fprintln(os.Stderr, "  "+flagColor.Sprint("record")+"         start recording")
}

func listDisplaysCmd() {
	displays, err := listDisplays()
	if err != nil {
		errorColor.Fprintf(os.Stderr, "Failed to list displays: %v\n", err)
		os.Exit(1)
	}
	for _, d := range displays {
		fmt.Println(d.String())
	}
	os.Exit(0)
}

func listSourcesCmd() {
	sources, err := listSources()
	if err != nil {
		errorColor.Fprintf(os.Stderr, "Failed to list sources: %v\n", err)
		os.Exit(1)
	}
	for _, s := range sources {
		fmt.Println(s.String())
	}
	os.Exit(0)
}

func recordCmd() {
	command := flag.NewFlagSet("record", flag.ExitOnError)

	var dname, sname, output, framerate, quality, bitrate string
	var help bool
	command.StringVar(&dname, "d", "", "name of display to record")
	command.StringVar(&dname, "display", "", "name of display to record")
	command.StringVar(&sname, "s", "", "audio source to record from")
	command.StringVar(&sname, "source", "", "audio source to record from")
	defaultOutput := "."
	if envDir := os.Getenv("CAPSCREEN_OUTPUT_DIR"); envDir != "" {
		defaultOutput = envDir
	}
	command.StringVar(&output, "o", defaultOutput, "output directory for recording")
	command.StringVar(&output, "outdir", defaultOutput, "output directory for recording")
	command.StringVar(&framerate, "fr", "30", "frame rate for recording")
	command.StringVar(&quality, "quality", "18", "video quality (0-51, lower = higher quality)")
	command.StringVar(&bitrate, "bitrate", "192k", "audio bitrate")
	command.BoolVar(&help, "h", false, "show usage")
	command.BoolVar(&help, "help", false, "show usage")
	command.Parse(os.Args[2:])

	if help {
		fmt.Fprintln(os.Stderr, infoColor.Sprint("usage:"), "capscreen record [-h|--help] [options]")
		fmt.Fprintln(os.Stderr, infoColor.Sprint("options:"))
		fmt.Fprintln(os.Stderr, "  "+flagColor.Sprint("-d, --display")+" <display>  name of display to record")
		fmt.Fprintln(os.Stderr, "  "+flagColor.Sprint("-s, --source")+"  <source>   audio source to record from")
		fmt.Fprintln(os.Stderr, "  "+flagColor.Sprint("-o, --outdir")+"  <dir>      output directory for recording")
		fmt.Fprintln(os.Stderr, "  "+flagColor.Sprint("--fr")+"          <fps>      frame rate for recording (default: 30)")
		fmt.Fprintln(os.Stderr, "  "+flagColor.Sprint("--quality")+"     <crf>      video quality 0-51, lower = higher quality (default: 18)")
		fmt.Fprintln(os.Stderr, "  "+flagColor.Sprint("--bitrate")+"     <rate>     audio bitrate (default: 192k)")
		fmt.Fprintln(os.Stderr, "  "+flagColor.Sprint("-h, --help")+"               show usage")
		os.Exit(0)
	}

	outpath, err := newRecordingFile(output)
	if err != nil {
		errorColor.Fprintf(os.Stderr, "Invalid output directory: %v\n", err)
		os.Exit(1)
	}

	xdisplay := os.Getenv("DISPLAY")
	if xdisplay == "" {
		errorColor.Fprintf(os.Stderr, "Failed to get $DISPLAY\n")
		os.Exit(1)
	}

	// always get a display
	var display *Display
	displays, err := listDisplays()
	if err != nil {
		errorColor.Fprintf(os.Stderr, "Failed to list displays: %v\n", err)
		os.Exit(1)
	}
	if dname == "" {
		display = &displays[0]
	} else {
		display = findItem(displays, dname)
	}
	if display == nil {
		errorColor.Fprintf(os.Stderr, "No such display: %s\n", dname)
		os.Exit(1)
	}

	// only get a source if one was specified
	var source *Source
	if sname != "" {
		sources, err := listSources()
		if err != nil {
			errorColor.Fprintf(os.Stderr, "Failed to list sources: %v\n", err)
			os.Exit(1)
		}
		source = findItem(sources, sname)
		if source == nil {
			errorColor.Fprintf(os.Stderr, "No such source: %s\n", sname)
			os.Exit(1)
		}
	}

	opts := RecordOpts{xdisplay, outpath, display, source, framerate, quality, bitrate}
	r, err := recordScreen(opts)
	if err != nil {
		errorColor.Fprintf(os.Stderr, "Failed to start recording: %v\n", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	timerDone := make(chan bool, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	start := time.Now()
	go updateTimer(start, timerDone)


	// Handle Ctrl+C signal
	go func() {
		<-sigChan
		timerDone <- true
		fmt.Fprint(os.Stderr, "\r", infoColor.Sprintf("recording: "), "stopped\n")
		r.stdin.Write([]byte("q\n"))
		r.stdin.Close()
		done <- true
	}()

	// Wait for either signal or natural completion
	go func() {
		err := r.cmd.Wait()
		timerDone <- true
		if err != nil && !strings.Contains(err.Error(), "exit status") {
			fmt.Fprintf(os.Stderr, "\n")
			errorColor.Fprintf(os.Stderr, "Recording failed: %v\n", err)
			errorColor.Fprintf(os.Stderr, "FFmpeg stderr: %s\n", r.stderr.String())
		}
		done <- true
	}()

	<-done
	abs, _ := filepath.Abs(opts.outpath)
	fmt.Println(abs)
	os.Exit(0)
}

func main() {
	var help bool
	flag.BoolVar(&help, "h", false, "show usage")
	flag.BoolVar(&help, "help", false, "show usage")
	flag.Parse()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	if help {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "list-displays":
		listDisplaysCmd()
	case "list-sources":
		listSourcesCmd()
	case "record":
		recordCmd()
	default:
		errorColor.Fprintf(os.Stderr, "Unknown command '%s'\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}
