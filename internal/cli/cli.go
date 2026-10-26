// Package cli 提供 DepScope 的命令行入口与无终端输出模式。
// 命令行与交互界面共用 app.Service，不重复实现解析、风险判断或报告组装。
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/render"
	"github.com/janghotan/depscope/internal/tui"
)

// IO 包含命令使用的输入输出流及其终端状态。
type IO struct {
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
	InTTY  bool
	OutTTY bool
	ErrTTY bool
}

// Options 配置一次命令行调用。
type Options struct {
	Service          app.Service
	IO               IO
	Args             []string
	Version          string
	ColorMode        string
	DefaultEcosystem model.Ecosystem
	LookupEnv        func(string) string
}

// Run 执行命令并返回进程退出码：0 成功，1 操作失败，2 用法错误，130 中断。
func Run(ctx context.Context, opts Options) int {
	out := opts.IO.Out
	if out == nil {
		out = io.Discard
	}
	errOut := opts.IO.Err
	if errOut == nil {
		errOut = io.Discard
	}
	lookupEnv := opts.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.Getenv
	}
	version := opts.Version
	if version == "" {
		version = "dev"
	}
	defaultEcosystem := opts.DefaultEcosystem
	if !defaultEcosystem.IsValid() {
		defaultEcosystem = model.EcosystemNPM
	}

	flags, code := parseGlobalFlags(opts.Args, opts.ColorMode, errOut)
	if code >= 0 {
		return code
	}
	if flags.help {
		printHelp(out)
		return 0
	}
	if flags.version {
		printVersion(out, version)
		return 0
	}
	colorMode := flags.mode
	args := flags.args

	if len(args) == 0 {
		interactive := opts.IO.InTTY && opts.IO.OutTTY && !strings.EqualFold(lookupEnv("TERM"), "dumb")
		if !interactive {
			printHelp(out)
			return 0
		}
		service, err := ensureService(opts.Service)
		if err != nil {
			printError(errOut, err)
			return 1
		}
		err = tui.Run(ctx, service, tui.Options{
			ColorMode:        string(colorMode),
			DefaultEcosystem: defaultEcosystem,
			LookupEnv:        lookupEnv,
			Context:          ctx,
			// 留空表示由 DEPSCOPE_NO_ANIMATION、TERM=dumb 与 CI 环境决定。
			Animations: "",
		})
		if err != nil {
			if errors.Is(err, tea.ErrInterrupted) || errors.Is(err, context.Canceled) {
				return 130
			}
			printError(errOut, err)
			return 1
		}
		return 0
	}

	service, err := ensureService(opts.Service)
	if err != nil {
		printError(errOut, err)
		return 1
	}
	environment := commandEnvironment{
		ctx:       ctx,
		service:   service,
		out:       out,
		errOut:    errOut,
		colorMode: colorMode,
		lookupEnv: lookupEnv,
		outTTY:    opts.IO.OutTTY,
		ecosystem: defaultEcosystem,
	}

	command := args[0]
	rest := args[1:]
	switch command {
	case "help":
		printHelp(out)
		return 0
	case "version":
		printVersion(out, version)
		return 0
	case "scan":
		return environment.scan(rest)
	case "project":
		return environment.project(rest)
	case "compare":
		return environment.compare(rest)
	case "inventory":
		return environment.inventory(rest)
	case "demo":
		return environment.demo(rest)
	default:
		fmt.Fprintf(errOut, "depscope: unknown command %q\n", render.Sanitize(command))
		fmt.Fprintln(errOut, "Run 'depscope help' to see available commands.")
		return 2
	}
}

type globalFlags struct {
	mode    render.ColorMode
	args    []string
	help    bool
	version bool
}

// parseGlobalFlags 解析命令前的全局选项；返回码 -1 表示继续执行命令。
func parseGlobalFlags(args []string, initial string, errOut io.Writer) (globalFlags, int) {
	mode, err := render.ParseColorMode(initial)
	if err != nil {
		printError(errOut, err)
		return globalFlags{mode: render.ColorAuto}, 2
	}
	rest := append([]string(nil), args...)
	for len(rest) > 0 {
		arg := rest[0]
		switch {
		case arg == "--no-color":
			mode = render.ColorNever
			rest = rest[1:]
		case arg == "--color":
			if len(rest) < 2 {
				printError(errOut, errors.New("--color requires auto, always, or never"))
				return globalFlags{mode: mode, args: rest}, 2
			}
			parsed, err := render.ParseColorMode(rest[1])
			if err != nil {
				printError(errOut, err)
				return globalFlags{mode: mode, args: rest}, 2
			}
			mode = parsed
			rest = rest[2:]
		case strings.HasPrefix(arg, "--color="):
			parsed, err := render.ParseColorMode(strings.TrimPrefix(arg, "--color="))
			if err != nil {
				printError(errOut, err)
				return globalFlags{mode: mode, args: rest}, 2
			}
			mode = parsed
			rest = rest[1:]
		case arg == "--help" || arg == "-h":
			return globalFlags{mode: mode, help: true}, -1
		case arg == "--version" || arg == "-V":
			return globalFlags{mode: mode, version: true}, -1
		case arg == "--":
			return globalFlags{mode: mode, args: rest[1:]}, -1
		case strings.HasPrefix(arg, "-"):
			printError(errOut, fmt.Errorf("unknown flag %q", arg))
			return globalFlags{mode: mode, args: rest}, 2
		default:
			return globalFlags{mode: mode, args: rest}, -1
		}
	}
	return globalFlags{mode: mode, args: rest}, -1
}

func ensureService(service app.Service) (app.Service, error) {
	if service != nil {
		return service, nil
	}
	built, err := app.NewLiveService(app.LiveOptions{})
	if err != nil {
		return nil, fmt.Errorf("create scan service: %w", err)
	}
	return built, nil
}

type commandEnvironment struct {
	ctx       context.Context
	service   app.Service
	out       io.Writer
	errOut    io.Writer
	colorMode render.ColorMode
	lookupEnv func(string) string
	outTTY    bool
	ecosystem model.Ecosystem
}

func (e commandEnvironment) theme() render.Theme {
	return render.Theme{
		Color: render.ColorEnabled(e.colorMode, e.outTTY, e.lookupEnv),
		Width: outputWidth(e.lookupEnv),
	}
}

func (e commandEnvironment) scan(args []string) int {
	if len(args) != 1 {
		return e.usage("usage: depscope scan <package>", "Example: depscope scan lodash@4.17.21")
	}
	request, err := app.ParsePackageSpec(args[0], e.ecosystem)
	if err != nil {
		return e.usage("invalid package: "+render.Sanitize(err.Error()), "Use lodash@4.17.21, @scope/pkg, or pypi:requests==2.31.0.")
	}
	report, err := e.service.ScanPackage(e.ctx, request, nil)
	if err != nil {
		printError(e.errOut, err)
		return 1
	}
	if err := writeDocument(e.out, render.FromReport(report), e.theme()); err != nil {
		printError(e.errOut, err)
		return 1
	}
	return statusExitCode(report.Status)
}

func (e commandEnvironment) project(args []string) int {
	if len(args) > 1 {
		return e.usage("usage: depscope project [path]", "The path defaults to the current directory.")
	}
	path := "."
	if len(args) == 1 {
		path = args[0]
	}
	request := app.ProjectRequest{Path: path}
	if err := request.Validate(); err != nil {
		return e.usage(render.Sanitize(err.Error()), "Pass a project directory, for example: depscope project .")
	}
	result, err := e.service.ScanProject(e.ctx, request, nil)
	if err != nil {
		printError(e.errOut, err)
		return 1
	}
	if err := writeDocument(e.out, render.FromProject(result), e.theme()); err != nil {
		printError(e.errOut, err)
		return 1
	}
	return statusExitCode(result.Status)
}

func (e commandEnvironment) inventory(args []string) int {
	if len(args) != 0 {
		return e.usage("usage: depscope inventory", "The inventory is read-only and never installs or executes packages.")
	}
	result, err := e.service.ScanInventory(e.ctx, nil)
	if err != nil {
		printError(e.errOut, err)
		return 1
	}
	if err := writeDocument(e.out, render.FromInventory(result), e.theme()); err != nil {
		printError(e.errOut, err)
		return 1
	}
	return statusExitCode(result.Status)
}

func (e commandEnvironment) compare(args []string) int {
	if len(args) != 2 {
		return e.usage("usage: depscope compare <left> <right>", "Example: depscope compare lodash@4.17.21 pypi:requests==2.31.0")
	}
	left, err := app.ParsePackageSpec(args[0], e.ecosystem)
	if err != nil {
		return e.usage("invalid left package: "+render.Sanitize(err.Error()), "Both sides accept the same package syntax as 'depscope scan'.")
	}
	right, err := app.ParsePackageSpec(args[1], e.ecosystem)
	if err != nil {
		return e.usage("invalid right package: "+render.Sanitize(err.Error()), "Both sides accept the same package syntax as 'depscope scan'.")
	}
	request := app.CompareRequest{Left: left, Right: right}
	if err := request.Validate(); err != nil {
		return e.usage("invalid comparison: "+render.Sanitize(err.Error()), "")
	}
	result, err := e.service.Compare(e.ctx, request, nil)
	if err != nil {
		printError(e.errOut, err)
		return 1
	}
	text := render.Comparison(result.LeftRequest.String(), result.RightRequest.String(), result.Left, result.Right, result.LeftErr, result.RightErr, e.theme())
	if _, err := fmt.Fprintln(e.out, text); err != nil {
		printError(e.errOut, err)
		return 1
	}
	if result.Left == nil && result.Right == nil {
		return 1
	}
	return 0
}

func (e commandEnvironment) demo(args []string) int {
	if len(args) > 1 {
		return e.usage("usage: depscope demo [scenario|list]", "Run 'depscope demo list' to see available scenarios.")
	}
	scenarios := e.service.DemoScenarios()
	if len(args) == 1 && strings.EqualFold(args[0], "list") {
		fmt.Fprintln(e.out, "Offline demo scenarios")
		for _, scenario := range scenarios {
			fmt.Fprintf(e.out, "  %-16s %s\n", render.Sanitize(scenario.ID), render.Sanitize(scenario.Title))
			fmt.Fprintf(e.out, "  %-16s %s\n", "", render.Sanitize(scenario.Description))
		}
		return 0
	}
	scenario := "vulnerable"
	if len(args) == 1 {
		scenario = args[0]
	}
	if !scenarioExists(scenarios, scenario) {
		fmt.Fprintf(e.errOut, "depscope: unknown demo scenario %q\n", render.Sanitize(scenario))
		fmt.Fprintln(e.errOut, "Available scenarios:")
		for _, item := range scenarios {
			fmt.Fprintf(e.errOut, "  %s\n", render.Sanitize(item.ID))
		}
		return 2
	}
	report, err := e.service.Demo(e.ctx, scenario, nil)
	if err != nil {
		printError(e.errOut, err)
		return 1
	}
	if err := writeDocument(e.out, render.FromReport(report), e.theme()); err != nil {
		printError(e.errOut, err)
		return 1
	}
	return statusExitCode(report.Status)
}

func (e commandEnvironment) usage(message, hint string) int {
	fmt.Fprintf(e.errOut, "depscope: %s\n", render.Sanitize(message))
	if hint != "" {
		fmt.Fprintln(e.errOut, render.Sanitize(hint))
	}
	fmt.Fprintln(e.errOut, "Run 'depscope help' for usage.")
	return 2
}

func writeDocument(out io.Writer, document render.Document, theme render.Theme) error {
	_, err := fmt.Fprintln(out, document.Full(theme))
	return err
}

func outputWidth(lookupEnv func(string) string) int {
	if value := strings.TrimSpace(lookupEnv("COLUMNS")); value != "" {
		if width, err := strconv.Atoi(value); err == nil && width > 0 {
			return width
		}
	}
	return 100
}

func statusExitCode(status model.ScanStatus) int {
	if status == model.StatusFailed || status == model.StatusCancelled {
		return 1
	}
	return 0
}

func scenarioExists(scenarios []app.DemoScenario, id string) bool {
	for _, scenario := range scenarios {
		if scenario.ID == id {
			return true
		}
	}
	return false
}

func printError(out io.Writer, err error) {
	fmt.Fprintf(out, "depscope: %s\n", render.Sanitize(err.Error()))
}

func printVersion(out io.Writer, version string) {
	fmt.Fprintf(out, "depscope %s\n", render.Sanitize(version))
}

func printHelp(out io.Writer) {
	fmt.Fprint(out, helpText)
}

// IsTerminal 报告文件是否为字符设备；用于决定能否启动交互界面。
func IsTerminal(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

const helpText = `DepScope - dependency risk, explained.

Usage:
  depscope                          Open the interactive terminal app
  depscope scan <package>           Scan one npm or PyPI package
  depscope project [path]           Scan a project directory (default: .)
  depscope compare <left> <right>   Compare two packages side by side
  depscope inventory                List global npm and Python packages
  depscope demo [scenario]          Run an offline deterministic demo
  depscope demo list                List demo scenarios
  depscope help                     Show this help
  depscope version                  Show the version

Package syntax:
  lodash@4.17.21                    npm package with a version
  @scope/package                    scoped npm package
  pypi:requests==2.31.0             PyPI package with an exact version
  npm:lodash                        explicit ecosystem

Global flags (before the command):
  --no-color                        Disable ANSI color
  --color=auto|always|never         Control ANSI color

Without a terminal, depscope prints this help instead of starting the
interactive menu. NO_COLOR and TERM=dumb disable color.

DepScope reads metadata only. It never installs or executes the packages it
scans, and missing or failed checks never produce a clean result.
`
