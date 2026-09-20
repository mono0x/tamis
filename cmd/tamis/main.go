package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	applypkg "github.com/mono0x/tamis/internal/apply"
	"github.com/mono0x/tamis/internal/meta"
	"github.com/mono0x/tamis/internal/plan"
	"github.com/mono0x/tamis/internal/rule"
	"github.com/mono0x/tamis/internal/scan"
	"github.com/spf13/cobra"
)

type options struct {
	jobs int
	json bool
}

var version = "dev"

func main() {
	if err := newRoot().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRoot() *cobra.Command {
	opts := &options{}
	root := &cobra.Command{Use: "tamis", Version: version, SilenceUsage: true}
	root.PersistentFlags().IntVar(&opts.jobs, "jobs", 0, "metadata extraction workers (0 uses config or NumCPU)")
	root.PersistentFlags().BoolVar(&opts.json, "json", false, "write machine-readable output")
	root.AddCommand(newScanCommand(opts), newOperationCommand(opts, true), newOperationCommand(opts, false), newCheckCommand(), newExplainCommand(opts), newDoctorCommand())
	root.SetContext(context.Background())
	return root
}

func newOperationCommand(opts *options, dryRun bool) *cobra.Command {
	name, short := "apply", "validate and execute media operations"
	if dryRun {
		name, short = "preview", "preview validated media operations without changing files"
	}
	var force bool
	var trashDir string
	cmd := &cobra.Command{Use: name + " CONFIG", Args: cobra.ExactArgs(1), Short: short, RunE: func(cmd *cobra.Command, args []string) error {
		compiled, err := rule.Load(args[0])
		if err != nil {
			return err
		}
		records, err := extractRules(cmd.Context(), compiled, opts.jobs, false)
		if err != nil {
			return err
		}
		p, err := compiled.Evaluate(records, time.Now())
		if err != nil {
			return err
		}
		applyOptions := applypkg.Options{Force: force, TrashDir: trashDir}
		validated, err := applypkg.Validate(p, applyOptions)
		if err != nil {
			return err
		}
		if opts.json {
			if err := plan.WriteNDJSON(cmd.OutOrStdout(), validated.Plan); err != nil {
				return err
			}
		} else {
			writeHumanPlan(cmd.OutOrStdout(), validated.Plan)
		}
		for _, warning := range validated.Warnings {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "warning:", warning)
		}
		if dryRun {
			return nil
		}
		result, err := validated.Run(cmd.Context(), applyOptions)
		for _, skip := range result.Skipped {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "skip %s %s → %s: %s\n", skip.Kind, skip.Source, skip.Dest, skip.Reason)
		}
		if err != nil {
			return err
		}
		if opts.json {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
				Type string `json:"type"`
				applypkg.Result
			}{"result", result})
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "applied %d operations\n", result.Completed)
		return nil
	}}
	cmd.Flags().BoolVar(&force, "force", false, "downgrade validation errors to warnings")
	cmd.Flags().StringVar(&trashDir, "trash-dir", "", "holding directory instead of the system trash")
	return cmd
}

func newScanCommand(opts *options) *cobra.Command {
	var config string
	cmd := &cobra.Command{Use: "scan [PATH]", Args: cobra.MaximumNArgs(1), Short: "scan and extract metadata", RunE: func(cmd *cobra.Command, args []string) error {
		compiled, err := loadRules(config, args)
		if err != nil {
			return err
		}
		records, extractErr := extractRules(cmd.Context(), compiled, opts.jobs, true)
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		for i := range records {
			if err := enc.Encode(&records[i]); err != nil {
				return err
			}
		}
		return extractErr
	}}
	cmd.Flags().StringVar(&config, "config", "", "configuration file (instead of PATH)")
	return cmd
}

func newCheckCommand() *cobra.Command {
	return &cobra.Command{Use: "check CONFIG", Args: cobra.ExactArgs(1), Short: "validate configuration", RunE: func(cmd *cobra.Command, args []string) error {
		_, err := rule.Load(args[0])
		if err == nil {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "configuration is valid")
		}
		return err
	}}
}

func newExplainCommand(opts *options) *cobra.Command {
	return &cobra.Command{Use: "explain CONFIG FILE", Args: cobra.ExactArgs(2), Short: "explain metadata and matching rules", RunE: func(cmd *cobra.Command, args []string) error {
		compiled, err := rule.Load(args[0])
		if err != nil {
			return err
		}
		path, err := filepath.Abs(args[1])
		if err != nil {
			return err
		}
		rel, ok := compiled.RelativePath(path)
		if !ok {
			return fmt.Errorf("file does not match any configured rule location")
		}
		record, err := provider(compiled).Extract(cmd.Context(), path)
		if err != nil {
			return err
		}
		record.RelPath = rel
		p, err := compiled.Evaluate([]meta.Record{*record}, time.Now())
		if err != nil {
			return err
		}
		if opts.json {
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(struct {
				Record *meta.Record `json:"record"`
				Plan   plan.Plan    `json:"plan"`
			}{record, p})
		}
		writeExplanation(cmd.OutOrStdout(), record, p)
		return nil
	}}
}

// loadRules compiles CONFIG, or synthesizes a single recursive scan scope for a
// bare PATH so that both forms carry the same settings defaults.
func loadRules(config string, args []string) (*rule.Compiled, error) {
	if len(args) == 1 {
		if config != "" {
			return nil, fmt.Errorf("PATH cannot be combined with --config")
		}
		path, err := filepath.Abs(args[0])
		if err != nil {
			return nil, err
		}
		return rule.New(&rule.Config{Version: 1, Rules: []rule.Rule{{Name: "scan", Location: path, Recursive: true}}})
	}
	if config == "" {
		return nil, fmt.Errorf("provide PATH or --config CONFIG")
	}
	return rule.Load(config)
}
func provider(c *rule.Compiled) meta.Chain {
	return meta.NewChain(c.Location, c.MVHDLocation)
}
func extractRules(ctx context.Context, c *rule.Compiled, jobs int, allMetadata bool) ([]meta.Record, error) {
	rules := c.Config.Rules
	if len(rules) == 0 {
		return nil, fmt.Errorf("no rules configured")
	}
	if jobs <= 0 {
		jobs = c.Config.Settings.Jobs
	}
	var candidates []scan.Candidate
	seenPaths := map[string]bool{}
	seenScopes := map[string]bool{}
	var errors []string
	for _, r := range rules {
		source := r.ScanSource()
		key, err := json.Marshal(source)
		if err != nil {
			return nil, err
		}
		if seenScopes[string(key)] {
			continue
		}
		seenScopes[string(key)] = true
		found, err := scan.Walk(source)
		if err != nil {
			errors = append(errors, err.Error())
		}
		for _, candidate := range found {
			if !seenPaths[candidate.Path] {
				seenPaths[candidate.Path] = true
				candidates = append(candidates, candidate)
			}
		}
	}
	chain := provider(c)
	if !allMetadata {
		chain.RequiredFields = c.RequiredFields
	}
	records, err := scan.Extract(ctx, candidates, chain, jobs, 10*time.Second)
	if err != nil {
		errors = append(errors, err.Error())
	}
	if len(errors) > 0 {
		return records, fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return records, nil
}
func writeHumanPlan(w io.Writer, p plan.Plan) {
	_, _ = fmt.Fprintf(w, "Plan %s (%d items, %d skipped)\n", p.CreatedAt.Format(time.RFC3339), len(p.Items), len(p.Skipped))
	for _, item := range p.Items {
		_, _ = fmt.Fprintf(w, "%s  [%s]\n", item.Source, item.Rule)
		writeOps(w, "  ", item.Ops)
	}
	for _, skip := range p.Skipped {
		_, _ = fmt.Fprintf(w, "skip %s: %s\n", skip.Source, skip.Reason)
	}
}
func writeExplanation(w io.Writer, r *meta.Record, p plan.Plan) {
	_, _ = fmt.Fprintln(w, r.Path)
	_, _ = fmt.Fprintf(w, "  format      %s\n", r.Format)
	if r.Captured != nil {
		_, _ = fmt.Fprintf(w, "  captured    %s\n    source    %s\n", r.Captured.Time.Format(time.RFC3339), r.Captured.Source)
	} else {
		_, _ = fmt.Fprintln(w, "  captured    unavailable")
	}
	if r.DateAdded != nil {
		_, _ = fmt.Fprintf(w, "  date_added  %s\n", r.DateAdded.Format(time.RFC3339))
	} else {
		_, _ = fmt.Fprintln(w, "  date_added  unavailable")
	}
	_, _ = fmt.Fprintf(w, "  model       %s\n", r.Model)
	for _, item := range p.Items {
		_, _ = fmt.Fprintf(w, "  matched     rule %q\n", item.Rule)
		writeOps(w, "    ", item.Ops)
	}
}

func writeOps(w io.Writer, indent string, ops []plan.Op) {
	for _, op := range ops {
		if op.Command != nil {
			_, _ = fmt.Fprintf(w, "%s%s\n%s  command\n", indent, op.Kind, indent)
			for _, line := range strings.Split(strings.TrimRight(op.Command.Run, "\n"), "\n") {
				_, _ = fmt.Fprintf(w, "%s    %s\n", indent, line)
			}
			for _, name := range slices.Sorted(maps.Keys(op.Command.Env)) {
				_, _ = fmt.Fprintf(w, "%s  env %s=%q\n", indent, name, op.Command.Env[name])
			}
			continue
		}
		if op.Kind == plan.Trash && op.Dest == "" {
			_, _ = fmt.Fprintf(w, "%s%-8s → system Trash\n", indent, op.Kind)
		} else if op.Dest == "" {
			_, _ = fmt.Fprintf(w, "%s%-8s\n", indent, op.Kind)
		} else {
			_, _ = fmt.Fprintf(w, "%s%-8s → %s\n", indent, op.Kind, op.Dest)
		}
	}
}

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{Use: "doctor CONFIG", Args: cobra.ExactArgs(1), Short: "diagnose configuration and providers", RunE: func(cmd *cobra.Command, args []string) error {
		compiled, err := rule.Load(args[0])
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "os/arch       %s/%s\n", runtime.GOOS, runtime.GOARCH)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "runtime exec  explicit shell actions only (/bin/sh)")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "providers     imagemeta, png-text, go-mp4, sidecar, filesystem")
		for _, r := range compiled.Config.Rules {
			info, statErr := os.Stat(r.Location)
			status := "ok"
			if statErr != nil {
				status = statErr.Error()
			} else if !info.IsDir() {
				status = "not a directory"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "rule %-12s %s (%s)\n", r.Name, r.Location, status)
		}
		return nil
	}}
}
