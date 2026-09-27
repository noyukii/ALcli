package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/noyukii/ALcli/internal/mcpserver"
)

func (s *commandState) mcpCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "mcp", Short: "Run the local ALcli MCP server"}
	cmd.AddCommand(&cobra.Command{Use: "stdio", Short: "Serve MCP over standard input and output", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return mcpserver.New().RunStdio(context.Background())
	}})
	var listen string
	httpCommand := &cobra.Command{Use: "http", Short: "Serve MCP on localhost with a bearer token", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return mcpserver.New().RunHTTP(ctx, listen, s.errOut)
	}}
	httpCommand.Flags().StringVar(&listen, "listen", "127.0.0.1:43820", "Local listen address (127.0.0.1:PORT).")
	cmd.AddCommand(httpCommand)
	cmd.AddCommand(&cobra.Command{Use: "token", Short: "Show the local HTTP MCP bearer token", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		token, err := mcpserver.Token()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(s.out, token)
		return err
	}})
	return cmd
}
