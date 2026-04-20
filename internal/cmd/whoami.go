package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Enverge-Labs/enverge-cli/internal/config"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the currently authenticated user",
	RunE:  runWhoami,
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

func runWhoami(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil || cfg.SessionToken == "" {
		return fmt.Errorf("not logged in. Run `enverge login --token <token>` to authenticate")
	}

	url := cfg.APIURL + "/v1/me"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.SessionToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach the server at %s: %w", cfg.APIURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("session expired or invalid. Run `enverge login --token <token>` to re-authenticate")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var user struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return fmt.Errorf("unexpected response from server: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Name:  %s\nEmail: %s\n", user.Name, user.Email)
	return nil
}
