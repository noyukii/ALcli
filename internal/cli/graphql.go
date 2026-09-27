package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/noyukii/ALcli/internal/api"
)

func (s *commandState) emitData(value any) error {
	encoder := json.NewEncoder(s.out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}

func (s *commandState) graphqlCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "graphql", Short: "Run AniList GraphQL documents"}
	for _, kind := range []string{"query", "mutation"} {
		kind := kind
		var path, document, variables string
		sub := &cobra.Command{Use: kind, Short: "Run an AniList GraphQL " + kind, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if (path == "") == (document == "") {
				return errors.New("provide exactly one of --file or --document")
			}
			if path != "" {
				var raw []byte
				var err error
				if path == "-" {
					raw, err = io.ReadAll(io.LimitReader(os.Stdin, (1<<20)+1))
				} else {
					raw, err = os.ReadFile(path)
				}
				if err != nil {
					return fmt.Errorf("read GraphQL document: %w", err)
				}
				if len(raw) > 1<<20 {
					return errors.New("GraphQL document exceeds 1 MiB")
				}
				document = string(raw)
			}
			if api.DocumentKind(document) != kind {
				return fmt.Errorf("document must begin with a GraphQL %s", kind)
			}
			if kind == "mutation" && !s.config.IsAuthenticated() {
				return errors.New("mutation requires AniList login")
			}
			var input map[string]any
			if err := json.Unmarshal([]byte(variables), &input); err != nil || input == nil {
				return errors.New("--variables must be a JSON object")
			}
			client := newAPIClient(s.config.AccessToken)
			value, err := client.Execute(context.Background(), document, input)
			if err != nil {
				return err
			}
			return s.emitData(value)
		}}
		sub.Flags().StringVar(&path, "file", "", "GraphQL document path, or - for stdin.")
		sub.Flags().StringVar(&document, "document", "", "GraphQL document text.")
		sub.Flags().StringVar(&variables, "variables", "{}", "GraphQL variables as a JSON object.")
		cmd.AddCommand(sub)
	}
	return cmd
}

func (s *commandState) namedCommand(kind string) *cobra.Command {
	cmd := &cobra.Command{Use: kind, Short: "Browse AniList " + kind}
	if kind != "messages" && kind != "notifications" && kind != "follows" {
		cmd.AddCommand(&cobra.Command{Use: "get <id>", Short: "Get one " + kind + " record", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id < 1 {
				return errors.New("ID must be a positive integer")
			}
			client := newAPIClient(s.config.AccessToken)
			value, err := client.Named(context.Background(), kind, "get", api.NamedInput{ID: id})
			if err != nil {
				return err
			}
			return s.emitData(value)
		}})
	}
	var id, page, perPage int
	var search string
	list := &cobra.Command{Use: "list", Short: "List or search " + kind, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		client := newAPIClient(s.config.AccessToken)
		value, err := client.Named(context.Background(), kind, "list", api.NamedInput{ID: id, Search: strings.TrimSpace(search), Page: page, PerPage: perPage})
		if err != nil {
			return err
		}
		return s.emitData(value)
	}}
	list.Flags().IntVar(&id, "id", 0, "Related media, user, or messenger ID when supported.")
	list.Flags().StringVar(&search, "search", "", "Search text when supported.")
	list.Flags().IntVar(&page, "page", 1, "Result page.")
	list.Flags().IntVar(&perPage, "per-page", 20, "Results per page (1–50).")
	cmd.AddCommand(list)
	return cmd
}
