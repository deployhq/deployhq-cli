package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newTemplatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "templates",
		Aliases: []string{"template", "tmpl"},
		Short:   "Manage templates",
	}

	cmd.AddCommand(
		newTemplatesListCmd(),
		newTemplatesShowCmd(),
		newTemplatesPublicCmd(),
		newTemplatesPublicShowCmd(),
		newTemplatesCreateCmd(),
		newTemplatesUpdateCmd(),
		newTemplatesDeleteCmd(),
	)

	return cmd
}

func newTemplatesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			templates, err := client.ListTemplates(cliCtx.Background(), nil)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(templates,
					fmt.Sprintf("%d templates", len(templates)),
					output.Breadcrumb{Action: "create", Cmd: "dhq templates create --name <name>"},
					output.Breadcrumb{Action: "public", Cmd: "dhq templates public"},
				))
			}

			columns := []string{"Name", "Permalink", "Description"}
			rows := make([][]string, len(templates))
			for i, t := range templates {
				rows[i] = []string{t.Name, t.Permalink, t.Description}
			}
			env.WriteTable(columns, rows)

			if len(templates) > 0 {
				env.Status("\nTip: dhq projects create --name <name> --template %s", templates[0].Permalink)
			}
			return nil
		},
	}
}

func newTemplatesPublicCmd() *cobra.Command {
	var frameworkType string

	cmd := &cobra.Command{
		Use:   "public",
		Short: "List public templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			templates, err := client.ListPublicTemplates(cliCtx.Background(), frameworkType, nil)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(templates,
					fmt.Sprintf("%d public templates", len(templates)),
				))
			}

			columns := []string{"Name", "Permalink", "Description"}
			rows := make([][]string, len(templates))
			for i, t := range templates {
				rows[i] = []string{t.Name, t.Permalink, t.Description}
			}
			env.WriteTable(columns, rows)
			return nil
		},
	}

	cmd.Flags().StringVar(&frameworkType, "framework", "", "Filter by framework type (web_frameworks, cms, ecommerce, static_sites, all)")
	return cmd
}

func newTemplatesShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [permalink]",
		Short: "Show template details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			tmpl, err := client.GetTemplate(cliCtx.Background(), args[0])
			if err != nil {
				return err
			}

			return cliCtx.Envelope.WriteJSON(output.NewResponse(tmpl,
				fmt.Sprintf("Template: %s", tmpl.Name),
				output.Breadcrumb{Action: "update", Cmd: fmt.Sprintf("dhq templates update %s --name <name>", tmpl.Permalink)},
				output.Breadcrumb{Action: "delete", Cmd: fmt.Sprintf("dhq templates delete %s", tmpl.Permalink)},
			))
		},
	}
}

func newTemplatesPublicShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "public-show <template-id> <public-id>",
		Short: "Show a public template detail",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			tmpl, err := client.GetPublicTemplate(cliCtx.Background(), args[0], args[1])
			if err != nil {
				return err
			}

			return cliCtx.Envelope.WriteJSON(output.NewResponse(tmpl,
				fmt.Sprintf("Public template: %s", tmpl.Name),
			))
		},
	}
}

func newTemplatesCreateCmd() *cobra.Command {
	var (
		name              string
		description       string
		notificationEmail string
		emailNotify       bool
		projectID         string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new template",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return &output.UserError{Message: "Template name is required", Hint: "Use --name flag"}
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.TemplateCreateRequest{Name: name, Description: description}
			if notificationEmail != "" {
				req.NotificationEmail = notificationEmail
			}
			if cmd.Flags().Changed("email-notify") {
				req.EmailNotify = &emailNotify
			}
			if projectID != "" {
				req.ProjectID = projectID
			}

			tmpl, err := client.CreateTemplate(cliCtx.Background(), req)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(tmpl,
					fmt.Sprintf("Created template: %s", tmpl.Name),
				))
			}

			env.Status("Created template: %s (%s)", tmpl.Name, tmpl.Permalink)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Template name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Template description")
	cmd.Flags().StringVar(&notificationEmail, "notification-email", "", "Notification email address")
	cmd.Flags().BoolVar(&emailNotify, "email-notify", false, "Enable email notifications")
	cmd.Flags().StringVar(&projectID, "project", "", "Copy configuration from existing project (permalink or ID)")
	return cmd
}

func newTemplatesUpdateCmd() *cobra.Command {
	var name, description string

	cmd := &cobra.Command{
		Use:   "update [permalink]",
		Short: "Update a template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.TemplateUpdateRequest{}
			if cmd.Flags().Changed("name") {
				req.Name = name
			}
			if cmd.Flags().Changed("description") {
				req.Description = description
			}

			tmpl, err := client.UpdateTemplate(cliCtx.Background(), args[0], req)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(tmpl, fmt.Sprintf("Updated template: %s", tmpl.Name)))
			}
			env.Status("Updated template: %s", tmpl.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Template name")
	cmd.Flags().StringVar(&description, "description", "", "Template description")
	return cmd
}

func newTemplatesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [permalink]",
		Short: "Delete a template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			if err := client.DeleteTemplate(cliCtx.Background(), args[0]); err != nil {
				return err
			}

			cliCtx.Envelope.Status("Deleted template: %s", args[0])
			return nil
		},
	}
}
