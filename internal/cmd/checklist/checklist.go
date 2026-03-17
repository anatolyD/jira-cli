package checklist

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ankitpokhrel/jira-cli/api"
	"github.com/ankitpokhrel/jira-cli/internal/cmdutil"
	"github.com/ankitpokhrel/jira-cli/pkg/jira"
)

const (
	helpText = `Manage checklists on Jira issues.

Requires the "Checklist for Jira" plugin and the checklist custom field ID
to be configured in your .jira config file:

  checklist:
    field: customfield_10693

The field ID varies per Jira instance.`

	examples = `# List checklist items
$ jira checklist list ISSUE-123

# Add an item to the checklist
$ jira checklist add ISSUE-123 "Review the PR"

# Mark item #1 as done (1-based index)
$ jira checklist check ISSUE-123 1

# Mark item #2 as open
$ jira checklist uncheck ISSUE-123 2

# Remove item #3
$ jira checklist remove ISSUE-123 3`
)

// NewCmdChecklist is the checklist command.
func NewCmdChecklist() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "checklist <subcommand>",
		Short:   "Manage checklists on Jira issues",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"cl"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newCmdList(),
		newCmdAdd(),
		newCmdCheck(),
		newCmdUncheck(),
		newCmdRemove(),
	)

	return cmd
}

func getChecklistField() string {
	field := viper.GetString("checklist.field")
	if field == "" {
		cmdutil.Failed("Checklist custom field is not configured.\n" +
			"Add the following to your .jira config file:\n\n" +
			"  checklist:\n" +
			"    field: customfield_XXXXX\n")
	}
	return field
}

func getIssueKey(args []string) string {
	project := viper.GetString("project.key")
	return cmdutil.GetJiraIssueKey(project, args[0])
}

// --- list ---

func newCmdList() *cobra.Command {
	return &cobra.Command{
		Use:     "list ISSUE-KEY",
		Short:   "List checklist items",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(1),
		Run:     listChecklist,
	}
}

func listChecklist(cmd *cobra.Command, args []string) {
	field := getChecklistField()
	key := getIssueKey(args)

	debug, _ := cmd.Flags().GetBool("debug")
	client := api.DefaultClient(debug)

	items, err := client.GetChecklist(key, field)
	cmdutil.ExitIfError(err)

	if len(items) == 0 {
		fmt.Println("No checklist items found.")
		return
	}

	for i, item := range items {
		marker := "[ ]"
		if item.Status == "done" {
			marker = "[x]"
		}
		fmt.Printf("  %d. %s %s\n", i+1, marker, item.Text)
	}
}

// --- add ---

func newCmdAdd() *cobra.Command {
	return &cobra.Command{
		Use:   "add ISSUE-KEY \"item text\"",
		Short: "Add an item to the checklist",
		Args:  cobra.ExactArgs(2),
		Run:   addChecklistItem,
	}
}

func addChecklistItem(cmd *cobra.Command, args []string) {
	field := getChecklistField()
	key := getIssueKey(args)
	text := args[1]

	debug, _ := cmd.Flags().GetBool("debug")
	client := api.DefaultClient(debug)

	items, err := client.GetChecklist(key, field)
	cmdutil.ExitIfError(err)

	items = append(items, jira.ChecklistItem{Text: text, Status: "open"})

	err = client.SetChecklist(key, field, items)
	cmdutil.ExitIfError(err)

	cmdutil.Success("Item added to checklist on %s", key)
}

// --- check ---

func newCmdCheck() *cobra.Command {
	return &cobra.Command{
		Use:   "check ISSUE-KEY INDEX",
		Short: "Mark a checklist item as done",
		Args:  cobra.ExactArgs(2),
		Run:   checkItem,
	}
}

func checkItem(cmd *cobra.Command, args []string) {
	setItemStatus(cmd, args, "done")
}

// --- uncheck ---

func newCmdUncheck() *cobra.Command {
	return &cobra.Command{
		Use:   "uncheck ISSUE-KEY INDEX",
		Short: "Mark a checklist item as open",
		Args:  cobra.ExactArgs(2),
		Run:   uncheckItem,
	}
}

func uncheckItem(cmd *cobra.Command, args []string) {
	setItemStatus(cmd, args, "open")
}

func setItemStatus(cmd *cobra.Command, args []string, status string) {
	field := getChecklistField()
	key := getIssueKey(args)

	idx, err := strconv.Atoi(args[1])
	if err != nil || idx < 1 {
		cmdutil.Failed("INDEX must be a positive integer (1-based)")
		return
	}

	debug, _ := cmd.Flags().GetBool("debug")
	client := api.DefaultClient(debug)

	items, err := client.GetChecklist(key, field)
	cmdutil.ExitIfError(err)

	if idx > len(items) {
		cmdutil.Failed("Index %d out of range. Checklist has %d items.", idx, len(items))
		return
	}

	items[idx-1].Status = status

	err = client.SetChecklist(key, field, items)
	cmdutil.ExitIfError(err)

	action := "checked"
	if status == "open" {
		action = "unchecked"
	}
	cmdutil.Success("Item %d %s on %s: %s", idx, action, key, items[idx-1].Text)
}

// --- remove ---

func newCmdRemove() *cobra.Command {
	return &cobra.Command{
		Use:     "remove ISSUE-KEY INDEX",
		Short:   "Remove a checklist item",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(2),
		Run:     removeItem,
	}
}

func removeItem(cmd *cobra.Command, args []string) {
	field := getChecklistField()
	key := getIssueKey(args)

	idx, err := strconv.Atoi(args[1])
	if err != nil || idx < 1 {
		cmdutil.Failed("INDEX must be a positive integer (1-based)")
		return
	}

	debug, _ := cmd.Flags().GetBool("debug")
	client := api.DefaultClient(debug)

	items, err := client.GetChecklist(key, field)
	cmdutil.ExitIfError(err)

	if idx > len(items) {
		cmdutil.Failed("Index %d out of range. Checklist has %d items.", idx, len(items))
		return
	}

	removed := items[idx-1]
	items = append(items[:idx-1], items[idx:]...)

	err = client.SetChecklist(key, field, items)
	cmdutil.ExitIfError(err)

	_ = strings.TrimSpace(removed.Text)
	cmdutil.Success("Removed item %d from %s: %s", idx, key, removed.Text)
}
