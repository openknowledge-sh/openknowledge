package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func runUpgrade(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, upgradeHelpText())
		return 0
	}
	args = lifecycleFlagsAfterPath(args)
	flags := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	target := flags.String("to", "latest", "target OKF version")
	planOnly := flags.Bool("plan", false, "print the upgrade plan without writing")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(stderrOutput(), "upgrade accepts at most one knowledge base path")
		return 2
	}
	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	}
	root, err := resolveWhereTarget(root)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	plan, err := okf.BuildUpgradePlan(root, *target)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	printUpgradePlan(plan)
	if len(plan.Changes) == 0 {
		fmt.Fprintln(os.Stdout, "Knowledge base is already up to date.")
		return 0
	}
	if *planOnly {
		return 0
	}
	if len(plan.SemanticIssues) > 0 {
		task := okf.UpgradeReviewTask(plan)
		if setupInputIsTerminal() {
			return continueReviewTask(bufio.NewReader(setupInput), root, task)
		}
		fmt.Print(task)
		return 1
	}
	if err := okf.ApplyUpgradePlan(plan); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if plan.From == plan.To {
		fmt.Fprintf(os.Stdout, "Updated managed artifacts for %s at OKF %s.\n", plan.Root, plan.To)
	} else {
		fmt.Fprintf(os.Stdout, "Upgraded %s from OKF %s to OKF %s.\n", plan.Root, plan.From, plan.To)
	}
	return 0
}

func printUpgradePlan(plan okf.UpgradePlan) {
	fmt.Fprintln(os.Stdout, "Upgrade plan")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "%-18s%s\n", "Knowledge base:", plan.Root)
	fmt.Fprintf(os.Stdout, "%-18s%s\n", "Current OKF:", plan.From)
	fmt.Fprintf(os.Stdout, "%-18s%s\n", "Target OKF:", plan.To)
	fmt.Fprintf(os.Stdout, "%-18s%d\n", "Mechanical changes:", len(plan.Changes))
	fmt.Fprintf(os.Stdout, "%-18s%d\n", "Semantic reviews:", len(plan.SemanticIssues))
	for _, change := range plan.Changes {
		fmt.Fprintf(os.Stdout, "  %-8s %-24s %s\n", strings.ToUpper(change.Action), change.Path, change.Reason)
	}
}

func upgradeHelpText() string {
	return `openknowledge upgrade

Migrate an existing knowledge base to a supported OKF version.

Usage:
  openknowledge upgrade [path] --plan
  openknowledge upgrade [path]
  openknowledge upgrade [path] --to <version>

Mechanical migrations preserve content and use atomic writes. Semantic issues
produce an agent review task before any migration files are changed.
`
}
