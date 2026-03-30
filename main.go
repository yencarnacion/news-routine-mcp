package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "0.1.0"

func main() {
	mode, configPath, envFile, err := parseCLI(os.Args[1:])
	if err != nil {
		printUsage(os.Stderr)
		log.Fatal(err)
	}

	app, err := newApp(configPath, envFile)
	if err != nil {
		log.Fatal(err)
	}

	switch mode {
	case "stdio":
		if err := runStdio(app); err != nil {
			log.Fatal(err)
		}
	default:
		if err := runHTTP(app); err != nil {
			log.Fatal(err)
		}
	}
}

func parseCLI(args []string) (mode string, configPath string, envFile string, err error) {
	mode = "serve"
	configPath = "config.yaml"
	envFile = ".env"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "serve", "stdio":
			mode = args[i]
		case "-config", "--config":
			i++
			if i >= len(args) {
				return "", "", "", errors.New("missing value for --config")
			}
			configPath = args[i]
		case "-env-file", "--env-file":
			i++
			if i >= len(args) {
				return "", "", "", errors.New("missing value for --env-file")
			}
			envFile = args[i]
		case "-h", "--help", "help":
			printUsage(os.Stdout)
			os.Exit(0)
		default:
			return "", "", "", fmt.Errorf("unknown argument: %s", args[i])
		}
	}

	return mode, configPath, envFile, nil
}

func printUsage(w *os.File) {
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  go run .                  # serve streamable HTTP MCP on config.yaml port")
	_, _ = fmt.Fprintln(w, "  go run . serve            # same as above")
	_, _ = fmt.Fprintln(w, "  go run . stdio            # run as a stdio MCP server")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Options:")
	_, _ = fmt.Fprintln(w, "  --config <path>           YAML config file (default: config.yaml)")
	_, _ = fmt.Fprintln(w, "  --env-file <path>         dotenv file (default: .env)")
}

func runHTTP(app *App) error {
	addr := app.Config.Server.Host + ":" + strconv.Itoa(app.Config.Server.Port)
	mux := http.NewServeMux()
	mux.Handle(app.Config.Server.Path, app.HTTPHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(
			w,
			"news-routine-mcp %s\nMCP endpoint: http://%s%s\nHealth: http://%s/healthz\n",
			version,
			addr,
			app.Config.Server.Path,
			addr,
		)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})

	app.Logger.Info(
		"starting streamable HTTP MCP server",
		slog.String("addr", addr),
		slog.String("path", app.Config.Server.Path),
		slog.String("openai_model", app.Config.Providers.OpenAI.Model),
		slog.String("grok_model", app.Config.Providers.Grok.Model),
	)

	return http.ListenAndServe(addr, mux)
}

func runStdio(app *App) error {
	app.Logger.Info("starting stdio MCP server")
	return app.Server.Run(context.Background(), &mcp.StdioTransport{})
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/mcp"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}
