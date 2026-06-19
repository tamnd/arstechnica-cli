package cli

import (
	"github.com/spf13/cobra"
)

// articleCmd returns a command that fetches articles for a fixed section.
func (a *App) articleCmd(name, short, section string, defaultLimit int) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(defaultLimit)
			a.progressf("fetching %s articles...", name)
			arts, err := a.client.Articles(cmd.Context(), section, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(arts, len(arts))
		},
	}
}

// feedCmd returns the `feed <section>` command.
func (a *App) feedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "feed <section>",
		Short: "Fetch articles from a named section",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			section := args[0]
			n := a.effectiveLimit(20)
			a.progressf("fetching %s articles...", section)
			arts, err := a.client.Articles(cmd.Context(), section, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(arts, len(arts))
		},
	}
}

// sectionsCmd returns the `sections` command.
func (a *App) sectionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sections",
		Short: "List available Ars Technica sections",
		RunE: func(cmd *cobra.Command, _ []string) error {
			secs := a.client.Sections()
			return a.renderOrEmpty(secs, len(secs))
		},
	}
}
