package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"
	"time"

	"github.com/Enverge-Labs/enverge-cli/internal/config"
	"github.com/spf13/cobra"
)

type nodeResponse struct {
	ID            string `json:"id"`
	Hostname      string `json:"hostname"`
	GPUModel      string `json:"gpu_model"`
	GPUCount      int    `json:"gpu_count"`
	RAMGB         int    `json:"ram_gb"`
	Status        string `json:"status"`
	LastHeartbeat string `json:"last_heartbeat"`
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered nodes",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil || cfg.SessionToken == "" {
		return fmt.Errorf("not logged in. Run `enverge login --token <token>` to authenticate")
	}

	url := cfg.APIURL + "/v1/nodes"
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

	var nodes []nodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		return fmt.Errorf("unexpected response from server: %w", err)
	}

	if len(nodes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no nodes)")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tHOSTNAME\tGPU\tSTATUS\tLAST HEARTBEAT")
	for _, n := range nodes {
		gpu := fmt.Sprintf("%s x%d %dGB", n.GPUModel, n.GPUCount, n.RAMGB)
		heartbeat := formatRelativeTime(n.LastHeartbeat)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", n.ID, n.Hostname, gpu, n.Status, heartbeat)
	}
	w.Flush()

	return nil
}

func formatRelativeTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
