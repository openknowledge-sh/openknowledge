package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type astOptions struct {
	path string
	out  string
	spec string
}

func runAST(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, astHelpText())
		return 0
	}
	options, err := parseASTOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	ast, err := okf.ParseASTWithVersion(root, options.spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	data, err := json.MarshalIndent(ast, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data = append(data, '\n')

	if options.out != "" {
		if err := writeOutputFileAtomically(options.out, data); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		terminal.success("Wrote AST")
		fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(ast.Root))
		fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(options.out))
		return 0
	}

	fmt.Print(string(data))
	return 0
}

func parseASTOptions(args []string) (astOptions, error) {
	options := astOptions{path: ".", spec: "latest"}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--out":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return astOptions{}, fmt.Errorf("--out requires a value")
			}
			options.out = args[index]
		case strings.HasPrefix(arg, "--out="):
			options.out = strings.TrimPrefix(arg, "--out=")
			if strings.TrimSpace(options.out) == "" {
				return astOptions{}, fmt.Errorf("--out requires a value")
			}
		case arg == "--spec":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return astOptions{}, fmt.Errorf("--spec requires a value")
			}
			options.spec = args[index]
		case strings.HasPrefix(arg, "--spec="):
			options.spec = strings.TrimPrefix(arg, "--spec=")
			if strings.TrimSpace(options.spec) == "" {
				return astOptions{}, fmt.Errorf("--spec requires a value")
			}
		case strings.HasPrefix(arg, "-"):
			return astOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			if options.path != "." {
				return astOptions{}, fmt.Errorf("ast accepts at most one path")
			}
			options.path = arg
		}
	}
	return options, nil
}

func astHelpText() string {
	return fmt.Sprintf(`openknowledge ast

Print the parsed Open Knowledge Format AST as JSON.

Usage:
  openknowledge ast [path]
  openknowledge ast --spec <version> [path]
  openknowledge ast --out <file> [path]
  openknowledge ast --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out       Output file. Defaults to stdout.
  --spec      OKF spec version. Defaults to latest.

Behavior:
  The AST output is the parser model before validation is converted into
  command-specific reports or export bundles. Per-document diagnostics remain
  attached to AST documents when file content, UTF-8, or frontmatter parsing
  fails.

Versions:
  %s
`, supportedSpecVersionsText())
}
