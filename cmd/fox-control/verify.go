package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	certIdentity = "https://github.com/fox-in-the-box-ai/fox-fleet/.github/workflows/release.yml@refs/tags/"
	certIssuer   = "https://token.actions.githubusercontent.com"
)

func newVerifyCmd() *cobra.Command {
	var sigFile, certFile string

	cmd := &cobra.Command{
		Use:   "verify <file>",
		Short: "Verify a cosign signature on a release artifact",
		Long: `Verify that a release artifact was signed by the Fox Fleet release pipeline.

Requires cosign to be installed (https://docs.sigstore.dev/cosign/system_config/installation/).

The signature (.sig) and certificate (.pem) files are expected alongside the artifact.
Override with --signature and --certificate flags.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			artifact := args[0]
			if _, err := os.Stat(artifact); err != nil {
				return fmt.Errorf("artifact not found: %s", artifact)
			}

			cosignPath, err := exec.LookPath("cosign")
			if err != nil {
				return fmt.Errorf("cosign not found in PATH — install it from https://docs.sigstore.dev/cosign/system_config/installation/")
			}

			if sigFile == "" {
				sigFile = artifact + ".sig"
			}
			if certFile == "" {
				certFile = artifact + ".pem"
			}

			if _, err := os.Stat(sigFile); err != nil {
				return fmt.Errorf("signature file not found: %s", sigFile)
			}
			if _, err := os.Stat(certFile); err != nil {
				return fmt.Errorf("certificate file not found: %s", certFile)
			}

			tag := extractTag(artifact)
			if tag == "dev" || tag == "unknown" || tag == "" {
				return fmt.Errorf("cannot determine release tag from filename %q — use cosign verify-blob directly with --certificate-identity", filepath.Base(artifact))
			}
			identity := certIdentity + tag

			verifyCmd := exec.CommandContext(cmd.Context(), cosignPath,
				"verify-blob",
				"--signature", sigFile,
				"--certificate", certFile,
				"--certificate-identity", identity,
				"--certificate-oidc-issuer", certIssuer,
				artifact,
			)
			verifyCmd.Stdout = cmd.OutOrStdout()
			verifyCmd.Stderr = cmd.ErrOrStderr()

			if err := verifyCmd.Run(); err != nil {
				return fmt.Errorf("signature verification failed: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Verified: %s\n", artifact)
			return nil
		},
	}
	cmd.Flags().StringVar(&sigFile, "signature", "", "signature file (default: <file>.sig)")
	cmd.Flags().StringVar(&certFile, "certificate", "", "certificate file (default: <file>.pem)")
	return cmd
}

func extractTag(filename string) string {
	base := strings.TrimSuffix(filepath.Base(filename), ".tar.gz")
	const prefix = "fox-control-"
	if !strings.HasPrefix(base, prefix) {
		return buildVersion
	}
	rest := base[len(prefix):]
	for _, goos := range []string{"linux", "darwin", "windows"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			suffix := "-" + goos + "-" + goarch
			if strings.HasSuffix(rest, suffix) {
				return rest[:len(rest)-len(suffix)]
			}
		}
	}
	return rest
}
