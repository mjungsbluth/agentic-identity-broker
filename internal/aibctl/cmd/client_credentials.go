package aibctlcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentic-identity-broker/agentic-identity-broker/internal/aibctl/client"
	"github.com/agentic-identity-broker/agentic-identity-broker/internal/aibctl/output"
)

func (c *cli) newClientCredentialsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "client-credentials",
		Aliases: []string{"creds"},
		Short:   "Manage the broker-issued client credentials of an agent",
	}
	completeAgents := c.completeResource(listAgentItems)

	get := &cobra.Command{
		Use:               "get AGENT",
		Short:             "Show credential metadata (never the secret)",
		Args:              exactArgs(1, "AGENT"),
		ValidArgsFunction: completeAgents,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := c.runtime(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := rt.Context(cmd)
			defer cancel()
			info, err := rt.Client.GetClientCredentials(ctx, args[0])
			if err != nil {
				return err
			}
			return rt.Printer.PrintObject(info, func() [][2]string {
				return [][2]string{
					{"Client ID", info.ClientID},
					{"Created", info.CreatedAt},
					{"Rotated", orDash(output.Deref(info.RotatedAt))},
				}
			})
		},
	}

	generate := func(use, short string, requireExisting bool) *cobra.Command {
		return &cobra.Command{
			Use:               use + " AGENT",
			Short:             short,
			Args:              exactArgs(1, "AGENT"),
			ValidArgsFunction: completeAgents,
			RunE: func(cmd *cobra.Command, args []string) error {
				rt, err := c.runtime(cmd)
				if err != nil {
					return err
				}
				ctx, cancel := rt.Context(cmd)
				defer cancel()
				if requireExisting {
					if _, err := rt.Client.GetClientCredentials(ctx, args[0]); err != nil {
						if client.IsStatus(err, 404) {
							return &client.APIError{Status: 404, Code: "not_found", Message: fmt.Sprintf("agent %q has no client credentials yet; use \"client-credentials create\"", args[0])}
						}
						return err
					}
				}
				creds, rotated, err := rt.Client.GenerateClientCredentials(ctx, args[0])
				if err != nil {
					return err
				}
				if rotated {
					_, _ = fmt.Fprintf(c.deps.Stderr, "Client credentials for %q rotated; the previous secret is invalid.\n", args[0])
				} else {
					_, _ = fmt.Fprintf(c.deps.Stderr, "Client credentials for %q created.\n", args[0])
				}
				_, _ = fmt.Fprintln(c.deps.Stderr, "The client secret is shown once and cannot be retrieved later.")
				return rt.Printer.PrintObject(creds, func() [][2]string {
					return [][2]string{
						{"Client ID", creds.ClientID},
						{"Client Secret", creds.ClientSecret},
						{"Created", creds.CreatedAt},
						{"Previous Invalidated", orDash(output.Deref(creds.PreviousInvalidatedAt))},
					}
				})
			},
		}
	}

	del := &cobra.Command{
		Use:               "delete AGENT",
		Aliases:           []string{"revoke", "rm"},
		Short:             "Revoke the agent's client credentials",
		Args:              exactArgs(1, "AGENT"),
		ValidArgsFunction: completeAgents,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := c.confirm(cmd, fmt.Sprintf("Revoke client credentials of agent %q", args[0])); err != nil {
				return err
			}
			rt, err := c.runtime(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := rt.Context(cmd)
			defer cancel()
			if err := rt.Client.RevokeClientCredentials(ctx, args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.deps.Stderr, "Client credentials of %q revoked.\n", args[0])
			return nil
		},
	}
	addYesFlag(del)

	cmd.AddCommand(
		get,
		generate("create", "Generate client credentials (rotates if they already exist)", false),
		generate("rotate", "Rotate existing client credentials", true),
		del,
	)
	return cmd
}
