package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"

	"github.com/Enverge-Labs/enverge-cli/internal/config"
	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Admin commands",
}

var adminApproveCmd = &cobra.Command{
	Use:   "approve <node-id>",
	Short: "Approve a pending node into the network",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdminApprove,
}

var adminListPendingCmd = &cobra.Command{
	Use:   "list-pending",
	Short: "List nodes waiting for approval",
	RunE:  runAdminListPending,
}

func init() {
	adminCmd.AddCommand(adminApproveCmd)
	adminCmd.AddCommand(adminListPendingCmd)
	rootCmd.AddCommand(adminCmd)
}

func runAdminApprove(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil || cfg.SessionToken == "" {
		return fmt.Errorf("not logged in. Run `enverge login --token <token>` to authenticate")
	}

	nodeID := args[0]

	// Resolve hostname to ID if needed
	if !looksLikeUUID(nodeID) {
		resolved, _, err := resolveNodeAllStatuses(cfg, nodeID)
		if err != nil {
			return err
		}
		nodeID = resolved
	}

	url := cfg.APIURL + "/v1/admin/nodes/" + nodeID + "/approve"
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.SessionToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach the server at %s: %w", cfg.APIURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("session expired or invalid. Run `enverge login --token <token>` to re-authenticate")
	}

	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("you don't have admin access")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to approve node (status %d): %s", resp.StatusCode, string(respBody))
	}

	var node struct {
		Hostname       string  `json:"hostname"`
		TunnelHostname *string `json:"tunnel_hostname"`
	}
	json.NewDecoder(resp.Body).Decode(&node)

	tunnel := ""
	if node.TunnelHostname != nil {
		tunnel = *node.TunnelHostname
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Node %s approved.\n", node.Hostname)
	if tunnel != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Tunnel: %s\n", tunnel)
	}
	return nil
}

type allNodeResponse struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	GPUModel string `json:"gpu_model"`
	GPUCount int    `json:"gpu_count"`
	RAMGB    int    `json:"ram_gb"`
	Status   string `json:"status"`
	Location string `json:"location"`
}

func runAdminListPending(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil || cfg.SessionToken == "" {
		return fmt.Errorf("not logged in. Run `enverge login --token <token>` to authenticate")
	}

	// Use the regular /v1/nodes endpoint but we need all nodes.
	// For now, use a new admin endpoint or filter client-side.
	// Let's call /v1/admin/nodes/pending
	url := cfg.APIURL + "/v1/admin/nodes/pending"
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

	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("you don't have admin access")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var nodes []allNodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		return fmt.Errorf("unexpected response from server: %w", err)
	}

	if len(nodes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no pending nodes)")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tHOSTNAME\tGPU\tLOCATION\tSTATUS")
	for _, n := range nodes {
		gpu := fmt.Sprintf("%s x%d %dGB", n.GPUModel, n.GPUCount, n.RAMGB)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", n.ID, n.Hostname, gpu, n.Location, n.Status)
	}
	w.Flush()

	return nil
}

// resolveNodeAllStatuses resolves a hostname to node ID across all statuses.
func resolveNodeAllStatuses(cfg config.Config, hostname string) (string, string, error) {
	url := cfg.APIURL + "/v1/admin/nodes/pending"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.SessionToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("could not reach the server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("could not fetch nodes")
	}

	var nodes []allNodeResponse
	json.NewDecoder(resp.Body).Decode(&nodes)

	for _, n := range nodes {
		if n.Hostname == hostname {
			return n.ID, n.Hostname, nil
		}
	}

	return "", "", fmt.Errorf("node with hostname %q not found in pending nodes", hostname)
}
