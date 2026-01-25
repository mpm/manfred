package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mpm/manfred/internal/config"
	"github.com/mpm/manfred/internal/github"
	"github.com/spf13/cobra"
)

func newGitHubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "github",
		Short: "GitHub integration commands",
		Long:  `Commands for managing GitHub integration, testing authentication, and webhook configuration.`,
	}

	cmd.AddCommand(newGitHubTestAuthCmd())
	cmd.AddCommand(newGitHubWebhookURLCmd())

	return cmd
}

func newGitHubTestAuthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test-auth",
		Short: "Verify GitHub credentials",
		Long:  `Tests that the configured GitHub token is valid by fetching the authenticated user.`,
		RunE:  runGitHubTestAuth,
	}
}

func runGitHubTestAuth(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.GitHub.Token == "" {
		fmt.Fprintln(os.Stderr, "Error: No GitHub token configured.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Set the token via:")
		fmt.Fprintln(os.Stderr, "  - Environment variable: GITHUB_TOKEN")
		fmt.Fprintln(os.Stderr, "  - Config file: github.token in config.yaml")
		return fmt.Errorf("no GitHub token configured")
	}

	client := github.NewClient(
		cfg.GitHub.Token,
		github.WithRateLimitBuffer(cfg.GitHub.RateLimitBuffer),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Testing GitHub authentication...")

	user, err := client.TestAuth(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Println()
	fmt.Printf("Authenticated as: %s\n", user.Login)
	fmt.Printf("User ID:          %d\n", user.ID)
	fmt.Printf("Profile:          %s\n", user.HTMLURL)

	// Show rate limit info if available
	if rl := client.GetRateLimit(); rl != nil {
		fmt.Println()
		fmt.Printf("Rate limit:       %d/%d (resets at %s)\n",
			rl.Remaining, rl.Limit, rl.Reset.Format(time.RFC3339))
	}

	fmt.Println()
	fmt.Println("GitHub authentication successful!")

	return nil
}

func newGitHubWebhookURLCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "webhook-url",
		Short: "Print the webhook URL for GitHub configuration",
		Long:  `Prints the webhook URL that should be configured in GitHub repository settings.`,
		RunE:  runGitHubWebhookURL,
	}
}

func runGitHubWebhookURL(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	addr := cfg.Server.Addr
	if addr == "0.0.0.0" || addr == "" {
		addr = "YOUR_SERVER_IP"
	}

	fmt.Printf("Webhook URL: http://%s:%d/webhook/github\n", addr, cfg.Server.Port)
	fmt.Println()
	fmt.Println("Configure this URL in your GitHub repository:")
	fmt.Println("  Settings > Webhooks > Add webhook")
	fmt.Println()
	fmt.Println("Required settings:")
	fmt.Println("  - Payload URL: <the URL above>")
	fmt.Println("  - Content type: application/json")
	fmt.Println("  - Secret: <your webhook secret>")
	fmt.Println("  - Events: Issues, Issue comments, Pull requests, Pull request reviews")

	if cfg.GitHub.WebhookSecret == "" {
		fmt.Println()
		fmt.Println("Warning: No webhook secret configured!")
		fmt.Println("Set MANFRED_WEBHOOK_SECRET or github.webhook_secret in config.yaml")
	}

	return nil
}
