package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/Enverge-Labs/enverge-cli/internal/cloudflared"
	"github.com/Enverge-Labs/enverge-cli/internal/config"
	"github.com/spf13/cobra"
)

type createSessionRequest struct {
	NodeID string `json:"node_id"`
}

type sshSessionResponse struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	Password       string `json:"password,omitempty"`
	Status         string `json:"status"`
	TunnelHostname string `json:"tunnel_hostname,omitempty"`
	NodeID         string `json:"node_id"`
}

var sshCmd = &cobra.Command{
	Use:   "ssh <node-id-or-hostname>",
	Short: "Start an SSH session to a node",
	Args:  cobra.ExactArgs(1),
	RunE:  runSSH,
}

func init() {
	sshCmd.Flags().Bool("print-command", false, "Print the SSH command instead of executing it")
	rootCmd.AddCommand(sshCmd)
}

func runSSH(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil || cfg.SessionToken == "" {
		return fmt.Errorf("not logged in. Run `enverge login --token <token>` to authenticate")
	}

	printCommand, _ := cmd.Flags().GetBool("print-command")
	nodeArg := args[0]

	// Resolve hostname to node ID if needed
	nodeID, hostname, err := resolveNode(cfg, nodeArg)
	if err != nil {
		return err
	}

	displayName := hostname
	if displayName == "" {
		displayName = nodeID
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Creating session on %s...\n", displayName)

	// Create the session
	session, err := createSession(cfg, nodeID)
	if err != nil {
		return err
	}

	// Poll until ready
	session, err = waitForSession(cmd, cfg, session.ID)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Session ready!")
	fmt.Fprintf(cmd.OutOrStdout(), "Username: %s\n", session.Username)
	fmt.Fprintf(cmd.OutOrStdout(), "Password: %s\n", session.Password)

	// For --print-command, use the expected path without requiring cloudflared to be installed
	if printCommand {
		cfPath, _ := cloudflared.BinPath()
		sshArgs := []string{
			"ssh",
			"-o", fmt.Sprintf("ProxyCommand=%s access tcp --hostname %s", cfPath, session.TunnelHostname),
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			fmt.Sprintf("%s@%s", session.Username, session.TunnelHostname),
		}
		fmt.Fprintln(cmd.OutOrStdout(), strings.Join(sshArgs, " "))
		return nil
	}

	// Ensure cloudflared is available for actual SSH
	cfPath, err := cloudflared.EnsureInstalled()
	if err != nil {
		return err
	}

	sshArgs := []string{
		"ssh",
		"-o", fmt.Sprintf("ProxyCommand=%s access tcp --hostname %s", cfPath, session.TunnelHostname),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		fmt.Sprintf("%s@%s", session.Username, session.TunnelHostname),
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Connecting via SSH...")

	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	return syscall.Exec(sshBin, sshArgs, os.Environ())
}

// resolveNode resolves a node argument to a node ID and hostname.
// If the argument looks like a UUID, it is used directly as the node ID.
// Otherwise, it is treated as a hostname and resolved via the API.
func resolveNode(cfg config.Config, nodeArg string) (nodeID string, hostname string, err error) {
	if looksLikeUUID(nodeArg) {
		return nodeArg, "", nil
	}

	// Treat as hostname — fetch nodes and find the match
	url := cfg.APIURL + "/v1/nodes"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.SessionToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("could not reach the server at %s: %w", cfg.APIURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", fmt.Errorf("session expired or invalid. Run `enverge login --token <token>` to re-authenticate")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var nodes []nodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		return "", "", fmt.Errorf("unexpected response from server: %w", err)
	}

	for _, n := range nodes {
		if n.Hostname == nodeArg {
			return n.ID, n.Hostname, nil
		}
	}

	return "", "", fmt.Errorf("node with hostname %q not found", nodeArg)
}

func looksLikeUUID(s string) bool {
	return len(s) == 36 && strings.Contains(s, "-")
}

func createSession(cfg config.Config, nodeID string) (sshSessionResponse, error) {
	body, _ := json.Marshal(createSessionRequest{NodeID: nodeID})
	url := cfg.APIURL + "/v1/sessions"

	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.SessionToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return sshSessionResponse{}, fmt.Errorf("could not reach the server at %s: %w", cfg.APIURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return sshSessionResponse{}, fmt.Errorf("session expired or invalid. Run `enverge login --token <token>` to re-authenticate")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return sshSessionResponse{}, fmt.Errorf("failed to create session (status %d): %s", resp.StatusCode, string(respBody))
	}

	var session sshSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return sshSessionResponse{}, fmt.Errorf("unexpected response from server: %w", err)
	}

	return session, nil
}

func waitForSession(cmd *cobra.Command, cfg config.Config, sessionID string) (sshSessionResponse, error) {
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	fmt.Fprintf(cmd.OutOrStdout(), "Waiting for session to be ready...")

	for {
		select {
		case <-timeout:
			fmt.Fprintln(cmd.OutOrStdout())
			return sshSessionResponse{}, fmt.Errorf("timed out waiting for session to become ready (60s)")
		case <-ticker.C:
			fmt.Fprintf(cmd.OutOrStdout(), ".")

			session, err := getSession(cfg, sessionID)
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout())
				return sshSessionResponse{}, err
			}

			switch session.Status {
			case "ready":
				fmt.Fprintln(cmd.OutOrStdout())
				return session, nil
			case "failed":
				fmt.Fprintln(cmd.OutOrStdout())
				return sshSessionResponse{}, fmt.Errorf("session failed to start")
			}
		}
	}
}

func getSession(cfg config.Config, sessionID string) (sshSessionResponse, error) {
	url := cfg.APIURL + "/v1/sessions/" + sessionID

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.SessionToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return sshSessionResponse{}, fmt.Errorf("could not reach the server at %s: %w", cfg.APIURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return sshSessionResponse{}, fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var session sshSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return sshSessionResponse{}, fmt.Errorf("unexpected response from server: %w", err)
	}

	return session, nil
}
