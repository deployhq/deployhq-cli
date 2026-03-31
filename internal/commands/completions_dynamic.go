package commands

import (
	"github.com/spf13/cobra"
)

// completeProjectNames provides dynamic completion for project names/permalinks.
func completeProjectNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	client, err := cliCtx.Client()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	projects, err := client.ListProjects(cliCtx.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, p := range projects {
		names = append(names, p.Permalink+"\t"+p.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeServerNames provides dynamic completion for server identifiers within a project.
//
//nolint:unused // Reserved for future use by server-specific commands.
func completeServerNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	projectID, err := cliCtx.RequireProject()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	client, err := cliCtx.Client()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	servers, err := client.ListServers(cliCtx.Background(), projectID)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, s := range servers {
		names = append(names, s.Identifier+"\t"+s.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
