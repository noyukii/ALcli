package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/noyukii/ALcli/internal/api"
	"github.com/spf13/cobra"
)

func (s *commandState) actionsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "action", Short: "Named AniList account changes"}
	for _, name := range api.NamedMutationNames() {
		name := name
		var variables string
		sub := &cobra.Command{Use: name, Short: api.NamedMutationHelp(name), Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if !s.config.IsAuthenticated() {
				return errors.New("account changes require AniList login")
			}
			var input map[string]any
			if err := json.Unmarshal([]byte(variables), &input); err != nil || input == nil {
				return fmt.Errorf("--variables must be a JSON object")
			}
			value, err := newAPIClient(s.config.AccessToken).NamedMutation(context.Background(), name, input)
			if err != nil {
				return err
			}
			return s.emitData(value)
		}}
		sub.Flags().StringVar(&variables, "variables", "{}", "Mutation arguments as a JSON object.")
		cmd.AddCommand(sub)
	}
	return cmd
}
