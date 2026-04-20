package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Enverge-Labs/enverge-cli/internal/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the Enverge control plane",
	RunE:  runLogin,
}

func init() {
	loginCmd.Flags().String("token", "", "API token")
	loginCmd.MarkFlagRequired("token")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	token, _ := cmd.Flags().GetString("token")

	body, _ := json.Marshal(map[string]string{"token": token})
	url := apiURL + "/v1/auth/login"

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("could not reach the server at %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		SessionToken string `json:"session_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("unexpected response from server: %w", err)
	}

	cfg := config.Config{
		APIURL:       apiURL,
		SessionToken: result.SessionToken,
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("Logged in successfully.")
	return nil
}
