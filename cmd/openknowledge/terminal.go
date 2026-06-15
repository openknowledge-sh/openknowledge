package main

import (
	"fmt"
	"os"
)

type terminalUI struct {
	color bool
}

func newTerminal(output *os.File) terminalUI {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return terminalUI{}
	}
	if os.Getenv("CLICOLOR_FORCE") != "" {
		return terminalUI{color: true}
	}

	info, err := output.Stat()
	if err != nil {
		return terminalUI{}
	}
	return terminalUI{color: info.Mode()&os.ModeCharDevice != 0}
}

func (t terminalUI) banner() {
	t.title("Open Knowledge", "CLI for managing open knowledge format v0.1 bundles")
}

func (t terminalUI) title(title string, subtitle string) {
	fmt.Println()
	fmt.Println(t.cyan(t.bold(title)))
	if subtitle != "" {
		fmt.Println(t.muted(subtitle))
	}
	fmt.Println()
}

func (t terminalUI) success(message string) {
	fmt.Printf("%s %s\n", t.green("OK"), t.bold(message))
}

func (t terminalUI) failure(message string) {
	fmt.Printf("%s %s\n", t.red("FAIL"), t.bold(message))
}

func (t terminalUI) section(message string) {
	fmt.Println(t.bold(message))
}

func (t terminalUI) status(value string) string {
	switch value {
	case "pass":
		return t.green("OK")
	case "warn":
		return t.yellow("WARN")
	case "fail":
		return t.red("FAIL")
	default:
		return value
	}
}

func (t terminalUI) path(value string) string {
	return t.cyan(value)
}

func (t terminalUI) green(value string) string {
	return t.wrap("32", value)
}

func (t terminalUI) cyan(value string) string {
	return t.wrap("36", value)
}

func (t terminalUI) yellow(value string) string {
	return t.wrap("33", value)
}

func (t terminalUI) red(value string) string {
	return t.wrap("31", value)
}

func (t terminalUI) bold(value string) string {
	return t.wrap("1", value)
}

func (t terminalUI) muted(value string) string {
	return t.wrap("90", value)
}

func (t terminalUI) wrap(code string, value string) string {
	if !t.color {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}
