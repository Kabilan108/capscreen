package main

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"

	// "os"
	"os/exec"
	"strings"

	// "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var ErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))

func main() {
	displays, err := getDisplays()
	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to get displays: %v", err)))
	}

	audioSources, err := getAudioSources()
	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to get audio sources: %v", err)))
	}

	fmt.Println("Available displays:")
	for i, d := range displays {
		fmt.Printf("%d - %s (%s, %s, %s)\n", i, d.Name, d.Resolution, d.OffsetX, d.OffsetY)
	}

	fmt.Println("\nAvailable audio sources:")
	for i, s := range audioSources {
		fmt.Printf("%d - %s (%s)\n", i, s.Name, s.Status)
	}
}

type Display struct {
	Name       string
	Resolution string
	OffsetX    string
	OffsetY    string
}

type AudioSource struct {
	Name   string
	Status string
}

func getDisplays() ([]Display, error) {
	monitorLines, err := runCmd("xrandr", "--listmonitors")
	if err != nil {
		return nil, err
	}
	geomLines, err := runCmd("xrandr", "--query")
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
	return displays, nil
}

func getAudioSources() ([]AudioSource, error) {
	lines, err := runCmd("pactl", "list", "sources", "short")
	if err != nil {
		return nil, err
	}
	var sources []AudioSource
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 7 {
			continue
		}
		sources = append(sources, AudioSource{Name: fields[1], Status: fields[6]})
	}
	return sources, nil
}

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func runCmd(name string, args ...string) ([]string, error) {
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
