package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// utils

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

func getDisplays() ([]Display, error) {
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

func getSources() ([]Source, error) {
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

// enums

type state int

const (
	selectDisplay state = iota
	selectSource
	recording
	done
)

// list.Item types

type Display struct{ Name, Resolution, OffsetX, OffsetY string }

func (d Display) Title() string       { return d.Name }
func (d Display) Description() string { return d.Resolution }
func (d Display) FilterValue() string { return d.Name }

type Source struct{ Name, Status string }

func (a Source) Title() string       { return a.Name }
func (a Source) Description() string { return a.Status }
func (a Source) FilterValue() string { return a.Name }

// bubbletea

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)
	errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
)

type (
	displayLoadedMsg []Display
	sourcesLoadedMsg []Source
)

type recordingMsg struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	err     error
	outFile string
}

type ffmpegDoneMsg struct {
	err error
}

type model struct {
	state       state
	xdisplay    string
	sd          string // selected display
	ss          string // selected source
	displays    list.Model
	sources     list.Model
	outFile     string
	ffmpegCmd   *exec.Cmd
	ffmpegStdin io.WriteCloser
	ffmpegErr   error
	cancelRec   context.CancelFunc
}

func initialModel(xdisplay, selDisplay, selSource, outDir string) model {
	dl := list.New([]list.Item{}, list.NewDefaultDelegate(), 200, 13)
	sl := list.New([]list.Item{}, list.NewDefaultDelegate(), 200, 13)
	dl.Title = "select display"
	sl.Title = "select audio source"
	return model{state: selectDisplay, xdisplay: xdisplay, displays: dl, sources: sl}
}

func loadDisplays() tea.Msg {
	disps, err := getDisplays()
	if err != nil {
		// TODO: figure out how to print error w bubbletea
		log.Fatal(err)
	}
	return displayLoadedMsg(disps)
}

func loadSources() tea.Msg {
	sources, err := getSources()
	if err != nil {
		log.Fatal(err)
	}
	return sourcesLoadedMsg(sources)
}

func startRecordingCmd(xdisplay string, d Display, s Source) tea.Cmd {
	return func() tea.Msg {
		output := fmt.Sprintf("%s.mp4", time.Now().Format("2006.01.02_03.04.05.mp4"))
		args := []string{
			"-f", "x11grab", "-video_size", d.Resolution, "-r", "30",
			"-i", fmt.Sprintf("%s.0+%s,%s", xdisplay, d.OffsetX, d.OffsetY),
			"-f", "pulse", "-i", s.Name,
			"-c:v", "libx264", "-preset", "ultrafast", "-crf", "18", "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-b:a", "192k", output,
		}

		cmd := exec.Command("ffmpeg", args...)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return recordingMsg{nil, nil, err, ""}
		}

		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			return recordingMsg{nil, nil, err, ""}
		}
		return recordingMsg{cmd, stdin, nil, output}
	}
}

func waitForFfmpegCmd(cmd *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		err := cmd.Wait()
		return ffmpegDoneMsg{err}
	}
}

func (m model) Init() tea.Cmd {
	return loadDisplays
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case selectDisplay:
		switch msg := msg.(type) {
		case displayLoadedMsg:
			items := make([]list.Item, len(msg))
			for i, d := range msg {
				items[i] = d
			}
			m.displays.SetItems(items)
			return m, loadSources
		case tea.KeyMsg:
			switch key := msg.String(); key {
			case "enter":
				if d, ok := m.displays.SelectedItem().(Display); ok {
					m.displays.Title = fmt.Sprintf("display: %s", d.Description())
					m.state = selectSource
					return m, loadSources
				}
			}
		}
		var cmd tea.Cmd
		m.displays, cmd = m.displays.Update(msg)
		return m, cmd

	case selectSource:
		switch msg := msg.(type) {
		case sourcesLoadedMsg:
			items := make([]list.Item, len(msg))
			for i, s := range msg {
				items[i] = s
			}
			m.sources.SetItems(items)
			return m, nil
		case tea.KeyMsg:
			switch key := msg.String(); key {
			case "enter":
				if src, ok := m.sources.SelectedItem().(Source); ok {
					disp := m.displays.SelectedItem().(Display)
					m.state = recording
					return m, startRecordingCmd(m.xdisplay, disp, src)
				}
			}
		}
		var cmd tea.Cmd
		m.sources, cmd = m.sources.Update(msg)
		return m, cmd

	case recording:
		switch msg := msg.(type) {
		case recordingMsg:
			m.ffmpegCmd = msg.cmd
			m.ffmpegStdin = msg.stdin
			m.outFile = msg.outFile
			if msg.err != nil {
				m.ffmpegErr = msg.err
				m.state = done
				return m, tea.Quit
			}
			return m, waitForFfmpegCmd(msg.cmd)
		case ffmpegDoneMsg:
			m.ffmpegErr = msg.err
			m.state = done
			return m, tea.Quit
		case tea.KeyMsg:
			if msg.String() == "ctrl+c" || msg.String() == "ctrl+d" {
				if m.ffmpegStdin != nil {
					io.WriteString(m.ffmpegStdin, "q\n")
					m.ffmpegStdin.Close()
					return m, tea.Quit
				}
			}
		}
		return m, nil

	case done:
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() string {
	switch m.state {
	case selectDisplay:
		return m.displays.View()
	case selectSource:
		return m.sources.View()
	case recording:
		return lipgloss.NewStyle().Width(50).Render(
			"Recording... press ctrl+c to stop\n",
		)
	case done:
		if m.ffmpegErr == nil {
			return fmt.Sprintf("✔  Saved as '%s'\n", m.outFile)
		}
		return fmt.Sprintf("✘  Error: %v\n", m.ffmpegErr)
	}
	return ""
}

// cli

var (
	selDisplay string
	selSource  string
	outDir     string
	help       bool
)

func printUsage() {
	fmt.Println(errStyle.Render("Usage: capscreen [-d|--display] [-s|--source] [-h|--help] [-o|--outdir]"))
}

func main() {
	flag.StringVar(&selDisplay, "d", "", "name of display to record")
	flag.StringVar(&selDisplay, "display", "", "name of display to record")
	flag.StringVar(&selSource, "s", "", "audio source to record from")
	flag.StringVar(&selSource, "source", "", "audio source to record from")
	flag.StringVar(&outDir, "o", "", "output directory for recording")
	flag.StringVar(&outDir, "outdir", "", "output directory for recording")
	flag.BoolVar(&help, "h", false, "show usage")
	flag.BoolVar(&help, "help", false, "show usage")

	if help {
		printUsage()
		os.Exit(1)
	}

	xdisplay := os.Getenv("DISPLAY")
	if xdisplay == "" {
		fmt.Println(errStyle.Render(fmt.Sprintf("Couldn't get $DISPLAY: '%s'", xdisplay)))
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel(xdisplay, selDisplay, selSource, outDir))
	if _, err := p.Run(); err != nil {
		fmt.Println(errStyle.Render(fmt.Sprintf("Something went wrong: %s", err)))
		os.Exit(1)
	}
}
