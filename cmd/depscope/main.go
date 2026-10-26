// Command depscope 是 DepScope 的终端入口。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/janghotan/depscope/internal/cli"
)

var version = "0.4.0-dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	code := cli.Run(ctx, cli.Options{
		IO: cli.IO{
			In:     os.Stdin,
			Out:    os.Stdout,
			Err:    os.Stderr,
			InTTY:  cli.IsTerminal(os.Stdin),
			OutTTY: cli.IsTerminal(os.Stdout),
			ErrTTY: cli.IsTerminal(os.Stderr),
		},
		Args:    os.Args[1:],
		Version: version,
	})
	os.Exit(code)
}
