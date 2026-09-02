package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kongken/mem0-mcp/cli/internal/client"
	"github.com/kongken/mem0-mcp/cli/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	initCmd.Flags().String("api-key", "", "existing API key (m0sk_...) to store")
	initCmd.Flags().String("base-url", "", "Mem0 OSS base URL")
	initCmd.Flags().String("user-id", "", "default user_id")
	initCmd.Flags().String("email", "", "log in / register with this email")
	initCmd.Flags().String("password", "", "password for --email (or set MEM0_PASSWORD)")
	initCmd.Flags().Bool("interactive", false, "force the interactive wizard")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up or update the CLI configuration",
	Long: `Walks through connecting this CLI to your Mem0 OSS server:

  1. base URL (default http://localhost:8888)
  2. authentication: paste an existing API key (m0sk_...), or log in / register
     with email + password and mint a fresh API key
  3. optional default user_id

Credentials are written to ~/.mem0/config.json (0600). You can skip the
wizard entirely with flags:

  mem0 init --api-key m0sk_xxx --base-url http://localhost:8888 --user-id alice
  mem0 init --email you@company.com --password '***'           # mints a key`,
	Args: cobra.NoArgs,
	RunE: runE("init", func(a *app, cmd *cobra.Command, _ []string) error {
		key, _ := cmd.Flags().GetString("api-key")
		baseURL, _ := cmd.Flags().GetString("base-url")
		userID, _ := cmd.Flags().GetString("user-id")
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		forceInteractive, _ := cmd.Flags().GetBool("interactive")

		// Providing credentials via flags is always non-interactive: prompts
		// cannot block CI or scripts. Only a bare `mem0 init` opens the wizard,
		// which aborts cleanly on EOF (e.g. closed stdin).
		if !forceInteractive && (key != "" || email != "") {
			return initNonInteractive(a, baseURL, key, email, password, userID)
		}
		return initWizard(a, baseURL, key, email, userID)
	}),
}

// initNonInteractive writes config from flags/env without prompting.
func initNonInteractive(a *app, baseURL, key, email, password, userID string) error {
	cfg := a.cfg
	cfg.BaseURL = strings.TrimRight(firstNonEmpty(baseURL, cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = config.DefaultBaseURL
	}
	if key != "" {
		cfg.APIKey = strings.TrimSpace(key)
	} else if email != "" {
		if password == "" {
			password = os.Getenv("MEM0_PASSWORD")
		}
		if password == "" {
			return fmt.Errorf("--email login needs --password or MEM0_PASSWORD (interactive mode: run `mem0 init`)")
		}
		apiKey, err := loginAndMintKey(a, cfg.BaseURL, email, password)
		if err != nil {
			return err
		}
		cfg.APIKey = apiKey
	}
	if userID != "" {
		cfg.UserID = userID
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	return verifyAndReport(a, cfg)
}

// initWizard drives the interactive setup.
func initWizard(a *app, baseURL, key, email, userID string) error {
	cfg := a.cfg
	rd := bufio.NewReader(os.Stdin)

	cur := cfg.BaseURL
	if baseURL != "" {
		cur = baseURL
	}
	var err error
	cur, err = prompt(rd, "Mem0 OSS base URL", cur)
	if err != nil {
		return wizardAbort(err)
	}
	cur = strings.TrimRight(cur, "/")
	if cur == "" {
		cur = config.DefaultBaseURL
	}

	var apiKey string
	if key != "" {
		apiKey = strings.TrimSpace(key)
	} else {
		choice, err := promptChoice(rd, "Authentication",
			[]string{"paste an existing API key (m0sk_...)", "log in or register with email + password"})
		if err != nil {
			return wizardAbort(err)
		}
		if choice == 2 {
			if email == "" {
				email, err = prompt(rd, "Email", "")
				if err != nil {
					return wizardAbort(err)
				}
			}
			password, err := promptSecret(rd, "Password")
			if err != nil {
				return wizardAbort(err)
			}
			minted, err := loginAndMintKey(a, cur, email, password)
			if err != nil {
				return err
			}
			apiKey = minted
		} else {
			apiKey, err = promptSecret(rd, "API key")
			if err != nil {
				return wizardAbort(err)
			}
		}
	}

	if userID == "" {
		var uid string
		uid, err = prompt(rd, "Default user_id (optional)", cfg.UserID)
		if err != nil {
			return wizardAbort(err)
		}
		userID = strings.TrimSpace(uid)
	}

	cfg.APIKey = apiKey
	cfg.BaseURL = cur
	cfg.UserID = userID
	if err := cfg.Save(); err != nil {
		return err
	}
	return verifyAndReport(a, cfg)
}

// loginAndMintKey registers an admin when the server has none, then logs in
// and creates an API key labelled "mem0-cli".
func loginAndMintKey(a *app, baseURL, email, password string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()

	c := client.New(baseURL, "", a.timeout)
	needsSetup, err := c.SetupStatus(ctx)
	if err == nil && needsSetup {
		name := firstNonEmpty(email, "admin")
		if _, err := c.Register(ctx, name, email, password); err != nil {
			return "", fmt.Errorf("register first admin: %w", err)
		}
	}
	tok, err := c.Login(ctx, email, password)
	if err != nil {
		return "", fmt.Errorf("login: %w", err)
	}
	c.Token = tok.AccessToken
	created, err := c.CreateAPIKey(ctx, "mem0-cli")
	if err != nil {
		return "", fmt.Errorf("create api key: %w", err)
	}
	return created.Key, nil
}

// verifyAndReport pings the server with the just-saved key and reports.
func verifyAndReport(a *app, cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()
	c := client.New(cfg.BaseURL, cfg.APIKey, a.timeout)
	if _, err := c.ServerConfig(ctx); err != nil {
		return fmt.Errorf("saved config, but verification failed: %w", err)
	}
	p, _ := config.Path()
	if a.agent {
		return a.emitAgent(0, map[string]any{
			"config_path": p,
			"base_url":    cfg.BaseURL,
			"api_key":     maskKey(cfg.APIKey),
			"verified":    true,
		})
	}
	fmt.Fprintf(a.stdout, "Connected to %s — %s\n", cfg.BaseURL, green("OK"))
	fmt.Fprintf(a.stdout, "Configuration written to %s\n", p)
	return nil
}

// ---- tiny input helpers (dependency-free wizard) ----
// Each helper returns an error on EOF so a closed/non-interactive stdin can
// never spin or silently proceed with empty answers.

func prompt(rd *bufio.Reader, label, def string) (string, error) {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, err := rd.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		if err != nil {
			return "", err
		}
		return def, nil
	}
	return line, nil
}

func promptSecret(rd *bufio.Reader, label string) (string, error) {
	fmt.Printf("%s: ", label)
	line, err := rd.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func promptChoice(rd *bufio.Reader, label string, options []string) (int, error) {
	fmt.Printf("%s:\n", label)
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}
	for {
		fmt.Printf("Choose [1-%d]: ", len(options))
		line, err := rd.ReadString('\n')
		if err != nil {
			return 0, err
		}
		var n int
		if _, scanErr := fmt.Sscanf(strings.TrimSpace(line), "%d", &n); scanErr == nil && n >= 1 && n <= len(options) {
			return n, nil
		}
	}
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// wizardAbort maps an EOF (closed/non-interactive stdin) to a friendly hint
// so the wizard can never hang or spin.
func wizardAbort(err error) error {
	if errors.Is(err, io.EOF) {
		return fmt.Errorf("init aborted: no input received (run `mem0 init` in a terminal, or pass --api-key / --email)")
	}
	return fmt.Errorf("init aborted: %w", err)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
