package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var addr string
	var port int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the web server",
		Long: `Start the MANFRED web server with admin UI.

Provides a REST API and web interface for managing jobs,
tickets, and projects.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement web server
			return fmt.Errorf("not implemented yet")
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1", "Address to listen on")
	cmd.Flags().IntVar(&port, "port", 8080, "Port to listen on")

	return cmd
}
