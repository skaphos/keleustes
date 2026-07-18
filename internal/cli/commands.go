/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/skaphos/keleustes/internal/api/openapi"
)

// notImplemented returns the standard scaffold error for unimplemented commands.
// Each subcommand keeps this single source of truth so the message stays
// consistent as features land.
func notImplemented(c *cobra.Command, _ []string) error {
	return fmt.Errorf("%s: not implemented in this scaffold", c.CommandPath())
}

func newAppCommand() *cobra.Command {
	app := &cobra.Command{
		Use:   "app",
		Short: "Inspect applications",
	}
	app.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List applications",
		RunE:  runAppList,
	})
	app.AddCommand(&cobra.Command{
		Use:   "get NAME",
		Short: "Show detail for one application",
		Args:  cobra.ExactArgs(1),
		RunE:  runAppGet,
	})
	return app
}

func runAppList(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	resp, err := client.GetApplicationsWithResponse(cmd.Context(), &openapi.GetApplicationsParams{})
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return apiError("list applications", resp.StatusCode(), resp.Body)
	}
	return renderApplications(cmd.OutOrStdout(), resp.JSON200.Items)
}

func runAppGet(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	resp, err := client.GetApplicationsNameWithResponse(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return apiError(fmt.Sprintf("get application %q", args[0]), resp.StatusCode(), resp.Body)
	}
	return renderApplications(cmd.OutOrStdout(), []openapi.Application{*resp.JSON200})
}

func newMatrixCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "matrix [APPLICATION]",
		Short: "Show the application/environment matrix",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runMatrix,
	}
	cmd.Flags().String("env", "", "Restrict the matrix to an environment")
	return cmd
}

func runMatrix(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	// "all" asks the read model for the whole fleet; a positional narrows
	// the view to one application's row.
	name := "all"
	if len(args) == 1 {
		name = args[0]
	}
	resp, err := client.GetApplicationsNameMatrixWithResponse(cmd.Context(), name)
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return apiError("matrix", resp.StatusCode(), resp.Body)
	}
	env, _ := cmd.Flags().GetString("env")
	return renderMatrix(cmd.OutOrStdout(), *resp.JSON200, env)
}

func newReleaseCommand() *cobra.Command {
	rel := &cobra.Command{
		Use:   "release",
		Short: "Inspect releases",
	}
	rel.AddCommand(&cobra.Command{
		Use:   "list APPLICATION",
		Short: "List releases for an application",
		Args:  cobra.ExactArgs(1),
		RunE:  runReleaseList,
	})
	return rel
}

func runReleaseList(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	// Use the app-scoped GET /applications/{name}/releases rather than fetching
	// the fleet-wide GET /releases and filtering client-side.
	app := args[0]
	resp, err := client.GetApplicationsNameReleasesWithResponse(cmd.Context(), app)
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return apiError("list releases", resp.StatusCode(), resp.Body)
	}
	return renderReleases(cmd.OutOrStdout(), *resp.JSON200)
}

func newPromoteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "promote APPLICATION",
		Short: "Request a promotion for an application",
		Args:  cobra.ExactArgs(1),
		// Write path: POST /promotions is a server 501 until the Promotion
		// engine lands (MVP roadmap), so the CLI stays notImplemented for now.
		RunE: notImplemented,
	}
	cmd.Flags().String("release", "", "Release name to promote")
	cmd.Flags().String("to", "", "Destination environment")
	cmd.Flags().String("cell", "", "Restrict the promotion to a cell")
	cmd.Flags().String("region", "", "Restrict the promotion to a region")
	cmd.Flags().String("change", "", "Change-management identifier (e.g., a CRQ ID)")
	cmd.Flags().Bool("pr", false, "Mutate Git via pull request (default: directCommit)")
	return cmd
}

func newDiffCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff APPLICATION",
		Short: "Diff an application across environments",
		Args:  cobra.ExactArgs(1),
		RunE:  runDiff,
	}
	cmd.Flags().String("from", "", "Source environment")
	cmd.Flags().String("to", "", "Destination environment")
	return cmd
}

func runDiff(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")

	// Two environments => env-env comparison; otherwise diff the app's Git
	// desired state against the live cluster (the default mode).
	params := &openapi.GetDiffParams{Mode: openapi.GitLive}
	left := args[0]
	right := ""
	if from != "" {
		left = from
	}
	if to != "" {
		right = to
	}
	if from != "" && to != "" {
		params.Mode = openapi.EnvEnv
	}
	if left != "" {
		params.Left = &left
	}
	if right != "" {
		params.Right = &right
	}

	resp, err := client.GetDiffWithResponse(cmd.Context(), params)
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return apiError("diff", resp.StatusCode(), resp.Body)
	}
	return renderDiff(cmd.OutOrStdout(), *resp.JSON200)
}

func newBlockersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blockers APPLICATION",
		Short: "Show promotion blockers for an application",
		Args:  cobra.ExactArgs(1),
		RunE:  runBlockers,
	}
	cmd.Flags().String("to", "", "Destination environment")
	return cmd
}

func runBlockers(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	// There is no dedicated blockers endpoint yet; the app-scoped promotions
	// list is the closest contract surface — fetch this application's promotions
	// and keep the blocked ones (narrowed to --to when given) client-side, rather
	// than pulling every blocked promotion fleet-wide.
	app := args[0]
	resp, err := client.GetApplicationsNamePromotionsWithResponse(cmd.Context(), app)
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return apiError("blockers", resp.StatusCode(), resp.Body)
	}
	to, _ := cmd.Flags().GetString("to")
	var matched []openapi.Promotion
	for _, p := range *resp.JSON200 {
		if p.Status != openapi.StatusBlocked {
			continue
		}
		if to != "" && p.To != to {
			continue
		}
		matched = append(matched, p)
	}
	return renderPromotions(cmd.OutOrStdout(), matched)
}

func newRollbackCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollback APPLICATION",
		Short: "Roll an application back to an earlier release",
		Args:  cobra.ExactArgs(1),
		// Write path: rollback re-promotes an older release through the same
		// server 501 write path as promote; kept notImplemented until it lands.
		RunE: notImplemented,
	}
	cmd.Flags().String("to-release", "", "Release to roll back to")
	cmd.Flags().String("env", "", "Environment to roll back")
	cmd.Flags().String("cell", "", "Cell to roll back")
	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print keleustesctl version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "keleustesctl 0.0.0 (scaffold)")
			return err
		},
	}
}

// --- rendering helpers ---------------------------------------------------
//
// The output.go table helpers are keyed on unstructured.Unstructured (the
// dynamic-client get/describe path). API-backed verbs render typed openapi
// models instead, so they share this small tabwriter writer that mirrors the
// same kubectl-style alignment.

func renderApplications(w io.Writer, apps []openapi.Application) error {
	rows := make([][]string, 0, len(apps))
	for _, a := range apps {
		rows = append(rows, []string{
			a.Name,
			derefString(a.Project),
			derefString(a.Owner),
			string(a.Status),
		})
	}
	return printTable(w, []string{"NAME", "PROJECT", "OWNER", "STATUS"}, rows)
}

func renderReleases(w io.Writer, rels []openapi.Release) error {
	rows := make([][]string, 0, len(rels))
	for _, r := range rels {
		rows = append(rows, []string{
			r.Version,
			r.App,
			formatTimePtr(r.Created),
			joinStrPtr(r.DeployedOn),
		})
	}
	return printTable(w, []string{"VERSION", "APP", "CREATED", "DEPLOYED-ON"}, rows)
}

func renderPromotions(w io.Writer, proms []openapi.Promotion) error {
	rows := make([][]string, 0, len(proms))
	for _, p := range proms {
		rows = append(rows, []string{
			p.Application,
			p.From,
			p.To,
			string(p.Status),
			derefString(p.Release),
		})
	}
	return printTable(w, []string{"APPLICATION", "FROM", "TO", "STATUS", "RELEASE"}, rows)
}

func renderDiff(w io.Writer, d openapi.Diff) error {
	if _, err := fmt.Fprintf(w, "mode=%s left=%s right=%s\n",
		d.Mode, derefString(d.Left), derefString(d.Right)); err != nil {
		return err
	}
	rows := make([][]string, 0, len(d.Entries))
	for _, e := range d.Entries {
		rows = append(rows, []string{e.Object, string(e.Change), boolPtrMark(e.Drift)})
	}
	if err := printTable(w, []string{"OBJECT", "CHANGE", "DRIFT"}, rows); err != nil {
		return err
	}
	if s := d.Summary; s != nil {
		if _, err := fmt.Fprintf(w, "\nsummary: +%d ~%d -%d\n",
			intPtrVal(s.Added), intPtrVal(s.Changed), intPtrVal(s.Removed)); err != nil {
			return err
		}
	}
	return nil
}

func renderMatrix(w io.Writer, m openapi.Matrix, envFilter string) error {
	// Resolve the ordered column set, optionally restricted to one env.
	type colKey struct{ env, region string }
	var cols []colKey
	headers := []string{"APPLICATION"}
	for _, c := range m.Columns {
		env := derefString(c.Env)
		if envFilter != "" && env != envFilter {
			continue
		}
		region := derefString(c.Region)
		cols = append(cols, colKey{env, region})
		headers = append(headers, matrixColumnHeader(env, region))
	}

	rows := make([][]string, 0, len(m.Rows))
	for _, r := range m.Rows {
		// Index a row's cells by env+region so they line up under the shared
		// headers even when a given cell is absent for this application.
		byKey := make(map[colKey]openapi.MatrixCell, len(r.Cells))
		for _, cell := range r.Cells {
			byKey[colKey{derefString(cell.Env), derefString(cell.Region)}] = cell
		}
		row := []string{r.Application}
		for _, ck := range cols {
			if cell, ok := byKey[ck]; ok {
				row = append(row, matrixCellText(cell))
				continue
			}
			row = append(row, "-")
		}
		rows = append(rows, row)
	}

	if _, err := fmt.Fprintf(w, "AS OF: %s\n", m.AsOf.Format(time.RFC3339)); err != nil {
		return err
	}
	return printTable(w, headers, rows)
}

func matrixColumnHeader(env, region string) string {
	if env == "" {
		env = "?"
	}
	if region == "" {
		return strings.ToUpper(env)
	}
	return strings.ToUpper(env + "/" + region)
}

func matrixCellText(c openapi.MatrixCell) string {
	s := string(c.Status)
	if s == "" {
		s = "-"
	}
	if c.Drift != nil && *c.Drift {
		s += "*" // drift marker
	}
	return s
}

// printTable writes a kubectl-style aligned table, mirroring output.go's
// renderTable so API-backed verbs look identical to the dynamic-client path.
func printTable(w io.Writer, headers []string, rows [][]string) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "No resources found.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer func() { _ = tw.Flush() }()

	if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := fmt.Fprintln(tw, strings.Join(r, "\t")); err != nil {
			return err
		}
	}
	return nil
}

// apiError turns a non-2xx response into a Go error, preferring the RFC 9457
// problem the server returns (ADR 0009) — its detail/title and `type` slug —
// over a bare status code.
func apiError(op string, status int, body []byte) error {
	var p openapi.Problem
	if err := json.Unmarshal(body, &p); err == nil && p.Type != "" {
		msg := p.Title
		if p.Detail != nil && *p.Detail != "" {
			msg = *p.Detail
		}
		return fmt.Errorf("%s: %s (type=%s, http %d)", op, msg, p.Type, status)
	}
	return fmt.Errorf("%s: unexpected status %d", op, status)
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func boolPtrMark(b *bool) string {
	if b != nil && *b {
		return "yes"
	}
	return ""
}

func intPtrVal(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func joinStrPtr(s *[]string) string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, ",")
}
