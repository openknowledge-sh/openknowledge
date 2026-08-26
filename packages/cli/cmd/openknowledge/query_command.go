package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type queryCommandOptions struct {
	path           string
	spec           string
	out            string
	query          string
	queryFile      string
	rulesFile      string
	profile        string
	text           string
	sparql         string
	sparqlFile     string
	datalogQuery   string
	embeddingURL   string
	embeddingModel string
	embeddingCache string
	access         []string
	limit          int
	timeout        time.Duration
}

const embeddingTokenEnv = "OPENKNOWLEDGE_EMBEDDING_TOKEN"

var queryEmbeddingHTTPClient *http.Client

func runQuery(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, queryHelpText())
		return 0
	}
	switch args[0] {
	case "sparql":
		return runQuerySPARQL(args[1:])
	case "datalog":
		return runQueryDatalog(args[1:])
	case "hybrid":
		return runQueryHybrid(args[1:])
	default:
		fmt.Fprintf(stderrOutput(), "unknown query engine: %s\n\n", args[0])
		fmt.Fprint(stderrOutput(), queryHelpText())
		return 2
	}
}

func runQuerySPARQL(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, querySPARQLHelpText())
		return 0
	}
	options, err := parseQueryCommandOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if err := validateQueryCommandOptions("sparql", options); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	query, err := queryCommandText(options.query, options.queryFile, 32<<10, "SPARQL query")
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	result, err := okf.QuerySPARQLWithVersion(context.Background(), root, options.spec, query, okf.SPARQLQueryOptions{
		AllowedAccess: options.access,
		Limits:        okf.SPARQLLimits{MaxResults: options.limit, Timeout: options.timeout},
	})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	return writeQueryCommandResult(options.out, result)
}

func runQueryDatalog(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, queryDatalogHelpText())
		return 0
	}
	options, err := parseQueryCommandOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if err := validateQueryCommandOptions("datalog", options); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	query, err := queryCommandText(options.query, options.queryFile, 32<<10, "Datalog query")
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	rules, err := queryCommandFile(options.rulesFile, 64<<10, "Datalog rules")
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	result, err := okf.QueryDatalogWithVersion(context.Background(), root, options.spec, okf.DatalogQuery{
		Query: query, Rules: rules, RuleProfile: options.profile,
	}, okf.DatalogQueryOptions{
		AllowedAccess: options.access,
		Limits:        okf.DatalogLimits{MaxResults: options.limit, Timeout: options.timeout},
	})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	return writeQueryCommandResult(options.out, result)
}

func runQueryHybrid(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, queryHybridHelpText())
		return 0
	}
	options, err := parseQueryCommandOptions(args)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if err := validateQueryCommandOptions("hybrid", options); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	sparqlQuery, err := queryCommandText(options.sparql, options.sparqlFile, 32<<10, "SPARQL query")
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	rules, err := queryCommandFile(options.rulesFile, 64<<10, "Datalog rules")
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	request := okf.HybridQuery{Text: options.text, SPARQL: sparqlQuery, Limit: options.limit}
	if strings.TrimSpace(options.datalogQuery) != "" {
		request.Datalog = &okf.DatalogQuery{Query: options.datalogQuery, Rules: rules, RuleProfile: options.profile}
	}
	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	embedding, embeddingCache, err := queryEmbeddingProvider(context.Background(), root, options)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	result, err := okf.QueryHybridWithVersion(context.Background(), root, options.spec, request, okf.HybridQueryOptions{
		AllowedAccess:  options.access,
		Embedding:      embedding,
		EmbeddingCache: embeddingCache,
		SPARQLLimits:   okf.SPARQLLimits{MaxResults: queryCandidateLimit(options.limit), Timeout: options.timeout},
		DatalogLimits:  okf.DatalogLimits{MaxResults: queryCandidateLimit(options.limit), Timeout: options.timeout},
	})
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	return writeQueryCommandResult(options.out, result)
}

func parseQueryCommandOptions(args []string) (queryCommandOptions, error) {
	model := strings.TrimSpace(os.Getenv("OPENKNOWLEDGE_EMBEDDING_MODEL"))
	if model == "" {
		model = okf.DefaultHTTPEmbeddingModel
	}
	options := queryCommandOptions{
		path: ".", spec: "latest", limit: 12, timeout: 2 * time.Second,
		embeddingURL:   strings.TrimSpace(os.Getenv("OPENKNOWLEDGE_EMBEDDING_URL")),
		embeddingModel: model,
		embeddingCache: strings.TrimSpace(os.Getenv("OPENKNOWLEDGE_EMBEDDING_CACHE")),
	}
	for index := 0; index < len(args); index++ {
		argument := args[index]
		name, value, inline := strings.Cut(argument, "=")
		next := func() (string, error) {
			if inline {
				if strings.TrimSpace(value) == "" {
					return "", fmt.Errorf("%s requires a value", name)
				}
				return value, nil
			}
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return "", fmt.Errorf("%s requires a value", name)
			}
			return args[index], nil
		}
		switch name {
		case "--out", "--spec", "--query", "--query-file", "--rules", "--profile", "--text", "--sparql", "--sparql-file", "--datalog-query", "--embedding-url", "--embedding-model", "--embedding-cache", "--access", "--limit", "--timeout":
			parsed, err := next()
			if err != nil {
				return queryCommandOptions{}, err
			}
			switch name {
			case "--out":
				options.out = parsed
			case "--spec":
				options.spec = parsed
			case "--query":
				options.query = parsed
			case "--query-file":
				options.queryFile = parsed
			case "--rules":
				options.rulesFile = parsed
			case "--profile":
				options.profile = parsed
			case "--text":
				options.text = parsed
			case "--sparql":
				options.sparql = parsed
			case "--sparql-file":
				options.sparqlFile = parsed
			case "--datalog-query":
				options.datalogQuery = parsed
			case "--embedding-url":
				options.embeddingURL = strings.TrimSpace(parsed)
			case "--embedding-model":
				options.embeddingModel = strings.TrimSpace(parsed)
			case "--embedding-cache":
				options.embeddingCache = strings.TrimSpace(parsed)
			case "--access":
				options.access = append(options.access, parsed)
			case "--limit":
				number, err := strconv.Atoi(parsed)
				if err != nil || number < 1 || number > 1000 {
					return queryCommandOptions{}, fmt.Errorf("--limit must be between 1 and 1000")
				}
				options.limit = number
			case "--timeout":
				duration, err := time.ParseDuration(parsed)
				if err != nil || duration <= 0 || duration > 30*time.Second {
					return queryCommandOptions{}, fmt.Errorf("--timeout must be a duration between 1ns and 30s")
				}
				options.timeout = duration
			}
		case "--help", "-h":
			return queryCommandOptions{}, errors.New("help flag must be handled before option parsing")
		default:
			if strings.HasPrefix(argument, "-") {
				return queryCommandOptions{}, fmt.Errorf("unknown flag: %s", argument)
			}
			if options.path != "." {
				return queryCommandOptions{}, fmt.Errorf("query accepts at most one path")
			}
			options.path = argument
		}
	}
	return options, nil
}

func validateQueryCommandOptions(engine string, options queryCommandOptions) error {
	if _, ok := okf.ResolveSpecVersion(options.spec); !ok {
		return fmt.Errorf("unsupported OKF spec version: %s", options.spec)
	}
	if options.query != "" && options.queryFile != "" {
		return errors.New("use either --query or --query-file, not both")
	}
	if options.sparql != "" && options.sparqlFile != "" {
		return errors.New("use either --sparql or --sparql-file, not both")
	}
	switch engine {
	case "sparql":
		if strings.TrimSpace(options.query) == "" && options.queryFile == "" {
			return errors.New("openknowledge query sparql requires --query <sparql> or --query-file <file>")
		}
		if options.rulesFile != "" || options.text != "" || options.sparql != "" || options.sparqlFile != "" || options.datalogQuery != "" || options.profile != "" {
			return errors.New("SPARQL query received Datalog or hybrid-only flags")
		}
	case "datalog":
		if strings.TrimSpace(options.query) == "" && options.queryFile == "" {
			return errors.New("openknowledge query datalog requires --query <atom> or --query-file <file>")
		}
		if options.text != "" || options.sparql != "" || options.sparqlFile != "" || options.datalogQuery != "" {
			return errors.New("Datalog query received hybrid-only flags")
		}
		if options.profile != "" && options.profile != okf.DatalogProfileSafe && options.profile != okf.DatalogProfileClosedWorld {
			return fmt.Errorf("unsupported Datalog rule profile: %s", options.profile)
		}
	case "hybrid":
		if options.query != "" || options.queryFile != "" {
			return errors.New("hybrid query uses --text, --sparql, and --datalog-query")
		}
		if strings.TrimSpace(options.text) == "" && strings.TrimSpace(options.sparql) == "" && options.sparqlFile == "" && strings.TrimSpace(options.datalogQuery) == "" {
			return errors.New("openknowledge query hybrid requires at least one text or structured query")
		}
		if options.rulesFile != "" && strings.TrimSpace(options.datalogQuery) == "" {
			return errors.New("--rules requires --datalog-query")
		}
		if options.profile != "" && strings.TrimSpace(options.datalogQuery) == "" {
			return errors.New("--profile requires --datalog-query")
		}
		if options.profile != "" && options.profile != okf.DatalogProfileSafe && options.profile != okf.DatalogProfileClosedWorld {
			return fmt.Errorf("unsupported Datalog rule profile: %s", options.profile)
		}
		if options.embeddingURL != "" && options.embeddingModel == "" {
			return errors.New("--embedding-url requires --embedding-model or OPENKNOWLEDGE_EMBEDDING_MODEL")
		}
		if options.embeddingURL == "" && options.embeddingCache != "" {
			return errors.New("--embedding-cache requires --embedding-url or OPENKNOWLEDGE_EMBEDDING_URL")
		}
	}
	return nil
}

func queryEmbeddingProvider(ctx context.Context, root string, options queryCommandOptions) (okf.EmbeddingProvider, string, error) {
	if strings.TrimSpace(options.embeddingURL) == "" {
		return nil, "", nil
	}
	provider, err := okf.NewHTTPEmbeddingProvider(ctx, okf.HTTPEmbeddingOptions{
		URL: options.embeddingURL, Model: options.embeddingModel,
		Token: strings.TrimSpace(os.Getenv(embeddingTokenEnv)), Client: queryEmbeddingHTTPClient,
	})
	if err != nil {
		return nil, "", err
	}
	cachePath := strings.TrimSpace(options.embeddingCache)
	if cachePath == "" {
		cachePath, err = okf.DefaultEmbeddingCachePath(root, provider.Model())
		if err != nil {
			return nil, "", err
		}
	}
	return provider, cachePath, nil
}

func queryCommandText(inline, path string, maxBytes int64, label string) (string, error) {
	if path == "" {
		if int64(len(inline)) > maxBytes {
			return "", fmt.Errorf("%s exceeds %d-byte limit", label, maxBytes)
		}
		return strings.TrimSpace(inline), nil
	}
	return queryCommandFile(path, maxBytes, label)
}

func queryCommandFile(path string, maxBytes int64, label string) (string, error) {
	if path == "" {
		return "", nil
	}
	content, err := okf.ReadFileAtMost(path, maxBytes)
	if err != nil {
		return "", fmt.Errorf("read %s file: %w", label, err)
	}
	return strings.TrimSpace(string(content)), nil
}

func writeQueryCommandResult(path string, result any) int {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	data = append(data, '\n')
	if path != "" {
		if err := writeOutputFileAtomically(path, data); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		return 0
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	return 0
}

func queryCandidateLimit(limit int) int {
	candidateLimit := limit * 5
	if candidateLimit < 50 {
		candidateLimit = 50
	}
	if candidateLimit > 1000 {
		candidateLimit = 1000
	}
	return candidateLimit
}
