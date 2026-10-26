package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janghotan/depscope/internal/app"
)

// Run 启动交互界面。调用方必须确认输入与输出都是终端。
func Run(ctx context.Context, service app.Service, opts Options) error {
	if opts.Context == nil {
		opts.Context = ctx
	}
	model := New(service, opts)
	program := tea.NewProgram(model, tea.WithContext(ctx), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
