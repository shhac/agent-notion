package cli

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/ids"
	v3 "github.com/shhac/agent-notion/internal/notion/v3"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

const (
	pagePollInterval      = 2 * time.Second
	workspacePollInterval = 5 * time.Second
	downloadTimeout       = 10 * time.Minute
	maxPollErrors         = 3
	maxPollBackoff        = 30 * time.Second
)

// registerExport wires the `export` command group (page, workspace, poll). All
// leaves are v3-only.
func registerExport(root *cobra.Command, g *GlobalFlags) {
	exp := &cobra.Command{
		Use:   "export",
		Short: "Export pages or workspace (v3 desktop session required)",
	}
	exp.AddCommand(exportPageCmd(g), exportWorkspaceCmd(g), exportPollCmd(g))
	addDomainUsage("export", exportUsageText)
	root.AddCommand(exp)
}

func exportPageCmd(g *GlobalFlags) *cobra.Command {
	var (
		format     string
		recursive  bool
		outputPath string
		waitSec    int
	)
	cmd := &cobra.Command{
		Use:   "page <page-id>",
		Short: "Export a page (and optionally subpages) to markdown or HTML",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := validateFormat(format)
			if err != nil {
				return err
			}
			if waitSec <= 0 {
				return output.New("--wait must be a positive number (seconds)", output.FixableByAgent)
			}
			pageID := ids.Normalize(args[0])
			res, err := withV3Client(g, func(c *v3.Client, _ *config.V3Session) (exportResult, error) {
				request := map[string]any{
					"block":     map[string]any{"id": pageID, "spaceId": c.SpaceID},
					"recursive": recursive,
					"exportOptions": map[string]any{
						"exportType":            f,
						"timeZone":              exportTimeZone(c),
						"locale":                "en",
						"flattenExportFiletree": false,
					},
					"shouldExportComments": false,
				}
				return exportAndDownload(cmd.Context(), g, c, "exportBlock", request,
					resolveOutput(outputPath), pagePollInterval, time.Duration(waitSec)*time.Second)
			})
			if err != nil {
				return err
			}
			return emitItem(g, map[string]any{
				"exported":       res.path,
				"format":         f,
				"pages_exported": res.pages,
				"recursive":      recursive,
			})
		},
	}
	cmd.Flags().StringVar(&format, "format", "markdown", "Export format: markdown or html")
	cmd.Flags().BoolVar(&recursive, "recursive", false, "Include subpages recursively")
	cmd.Flags().StringVar(&outputPath, "output", "", "Output file path (default: notion-export-<timestamp>.zip)")
	cmd.Flags().IntVar(&waitSec, "wait", 120, "Maximum time to wait for the export (seconds)")
	return cmd
}

func exportWorkspaceCmd(g *GlobalFlags) *cobra.Command {
	var (
		format     string
		outputPath string
		waitSec    int
	)
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Export the entire workspace to markdown or HTML",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := validateFormat(format)
			if err != nil {
				return err
			}
			if waitSec <= 0 {
				return output.New("--wait must be a positive number (seconds)", output.FixableByAgent)
			}
			res, err := withV3Client(g, func(c *v3.Client, _ *config.V3Session) (exportResult, error) {
				request := map[string]any{
					"spaceId": c.SpaceID,
					"exportOptions": map[string]any{
						"exportType": f,
						"timeZone":   exportTimeZone(c),
						"locale":     "en",
					},
					"shouldExportComments": false,
				}
				return exportAndDownload(cmd.Context(), g, c, "exportSpace", request,
					resolveOutput(outputPath), workspacePollInterval, time.Duration(waitSec)*time.Second)
			})
			if err != nil {
				return err
			}
			return emitItem(g, map[string]any{
				"exported":       res.path,
				"format":         f,
				"pages_exported": res.pages,
			})
		},
	}
	cmd.Flags().StringVar(&format, "format", "markdown", "Export format: markdown or html")
	cmd.Flags().StringVar(&outputPath, "output", "", "Output file path (default: notion-export-<timestamp>.zip)")
	cmd.Flags().IntVar(&waitSec, "wait", 600, "Maximum time to wait for the export (seconds)")
	return cmd
}

// exportPollCmd resumes an already-queued export task by ID and downloads it.
// (The TS printed a task ID on timeout but had no way to resume; this closes
// that gap.)
func exportPollCmd(g *GlobalFlags) *cobra.Command {
	var (
		outputPath string
		waitSec    int
	)
	cmd := &cobra.Command{
		Use:   "poll <task-id>",
		Short: "Poll a queued export task and download it once ready",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if waitSec <= 0 {
				return output.New("--wait must be a positive number (seconds)", output.FixableByAgent)
			}
			res, err := withV3Client(g, func(c *v3.Client, _ *config.V3Session) (exportResult, error) {
				return pollAndDownload(cmd.Context(), g, c, args[0],
					resolveOutput(outputPath), pagePollInterval, time.Duration(waitSec)*time.Second)
			})
			if err != nil {
				return err
			}
			return emitItem(g, map[string]any{
				"exported":       res.path,
				"pages_exported": res.pages,
			})
		},
	}
	cmd.Flags().StringVar(&outputPath, "output", "", "Output file path (default: notion-export-<timestamp>.zip)")
	cmd.Flags().IntVar(&waitSec, "wait", 600, "Maximum time to wait for the export (seconds)")
	return cmd
}

// --- Export orchestration ---

type exportResult struct {
	path  string
	pages int
}

// exportAndDownload enqueues an export task, polls to completion, and downloads
// the resulting zip. Progress goes to stderr so stdout stays clean.
func exportAndDownload(ctx context.Context, g *GlobalFlags, client *v3.Client, eventName string, request map[string]any, outputPath string, pollInterval, timeout time.Duration) (exportResult, error) {
	resp, err := client.EnqueueTask(ctx, v3.EnqueueTaskParams{EventName: eventName, Request: request})
	if err != nil {
		return exportResult{}, err
	}
	_, _ = fmt.Fprintf(g.stderr, "Export task queued: %s\n", resp.TaskID)
	return pollAndDownload(ctx, g, client, resp.TaskID, outputPath, pollInterval, timeout)
}

func pollAndDownload(ctx context.Context, g *GlobalFlags, client *v3.Client, taskID, outputPath string, pollInterval, timeout time.Duration) (exportResult, error) {
	task, err := pollTask(ctx, g, client, taskID, pollInterval, timeout)
	if err != nil {
		return exportResult{}, err
	}
	if task.State == "failure" {
		msg := "Export failed"
		if task.Error != "" {
			msg += ": " + task.Error
		} else {
			msg += ". Check the page ID and try again."
		}
		return exportResult{}, output.New(msg, output.FixableByHuman)
	}
	if task.Status == nil || task.Status.ExportURL == "" {
		return exportResult{}, output.New("Export succeeded but no download URL was provided.", output.FixableByRetry)
	}

	abs, err := filepath.Abs(outputPath)
	if err != nil {
		return exportResult{}, err
	}
	_, _ = fmt.Fprintln(g.stderr, "Downloading export...")
	if err := downloadFile(ctx, g.httpClient(), task.Status.ExportURL, abs); err != nil {
		return exportResult{}, err
	}
	return exportResult{path: abs, pages: task.Status.PagesExported}, nil
}

func pollTask(ctx context.Context, g *GlobalFlags, client *v3.Client, taskID string, pollInterval, timeout time.Duration) (v3.ExportTask, error) {
	deadline := time.Now().Add(timeout)
	lastPages := 0
	consecutiveErrors := 0

	for time.Now().Before(deadline) {
		resp, err := client.GetTasks(ctx, []string{taskID})
		if err != nil {
			if isTransientPollError(err) && consecutiveErrors < maxPollErrors {
				consecutiveErrors++
				backoff := min(pollInterval*time.Duration(1<<consecutiveErrors), maxPollBackoff)
				_, _ = fmt.Fprintf(g.stderr, "\nPoll error (attempt %d/%d), retrying in %ds...\n", consecutiveErrors, maxPollErrors, int(backoff.Seconds()))
				time.Sleep(backoff)
				continue
			}
			return v3.ExportTask{}, err
		}
		consecutiveErrors = 0

		if len(resp.Results) == 0 {
			return v3.ExportTask{}, output.New(fmt.Sprintf("Task %s not found in getTasks response.", taskID), output.FixableByRetry)
		}
		task := resp.Results[0]
		if task.State == "success" || task.State == "failure" {
			_, _ = fmt.Fprintln(g.stderr)
			return task, nil
		}
		if task.Status != nil && task.Status.PagesExported > lastPages {
			_, _ = fmt.Fprintf(g.stderr, "\rExporting... %d pages exported", task.Status.PagesExported)
			lastPages = task.Status.PagesExported
		}
		time.Sleep(pollInterval)
	}

	_, _ = fmt.Fprintf(g.stderr, "\nTask ID for manual follow-up: %s\n", taskID)
	return v3.ExportTask{}, output.New(
		fmt.Sprintf("Export timed out after %ds (task: %s). The export may still be running on Notion's servers.", int(timeout.Seconds()), taskID),
		output.FixableByRetry,
	).WithHint("run 'agent-notion export poll " + taskID + "' to resume")
}

// isTransientPollError reports whether a getTasks failure is worth retrying: a
// v3 5xx. (The TS also retried on JS network AbortError/TypeError; Go surfaces
// those as generic errors we do not special-case.)
func isTransientPollError(err error) bool {
	var httpErr *v3.HTTPError
	if stderrors.As(err, &httpErr) {
		return httpErr.Status >= 500
	}
	return false
}

func downloadFile(ctx context.Context, hc *http.Client, url, path string) error {
	if hc == nil {
		hc = http.DefaultClient
	}
	dctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(dctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return output.New(fmt.Sprintf("Download failed: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode)), output.FixableByRetry)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

func validateFormat(format string) (string, error) {
	if format == "markdown" || format == "html" {
		return format, nil
	}
	return "", output.New(fmt.Sprintf(`Invalid format %q. Use "markdown" or "html".`, format), output.FixableByAgent)
}

func resolveOutput(outputPath string) string {
	if outputPath != "" {
		return outputPath
	}
	return defaultExportFilename()
}

func defaultExportFilename() string {
	return "notion-export-" + time.Now().UTC().Format("2006-01-02T15-04-05") + ".zip"
}

func exportTimeZone(c *v3.Client) string {
	if c.UserTimeZone != "" {
		return c.UserTimeZone
	}
	return "UTC"
}

const exportUsageText = `agent-notion export — Export pages or workspace (v3 desktop session required)

SUBCOMMANDS:
  export page <page-id> [options]            Export a page to markdown or HTML
  export workspace [options]                 Export the entire workspace
  export poll <task-id> [options]            Resume/poll a queued export by task ID

PAGE OPTIONS:
  --format <format>    Export format: markdown or html (default: markdown)
  --recursive          Include subpages recursively (default: false)
  --output <path>      Output file path (default: notion-export-<timestamp>.zip)
  --wait <seconds>     Maximum wait time for the export (default: 120)

WORKSPACE / POLL OPTIONS:
  --format <format>    (workspace only) markdown or html (default: markdown)
  --output <path>      Output file path (default: notion-export-<timestamp>.zip)
  --wait <seconds>     Maximum wait time (workspace/poll default: 600)

OUTPUT:
  Page:      { exported, format, pages_exported, recursive }
  Workspace: { exported, format, pages_exported }
  Poll:      { exported, pages_exported }

  exported: Absolute path to the downloaded zip file.
  Progress is written to stderr during polling.

NOTES:
  Requires a v3 desktop session (auth import-desktop).
  Exports are asynchronous — the CLI polls until completion or the --wait
  timeout. On timeout the task ID is printed; resume it with 'export poll'.
  Large workspace exports may take several minutes.

EXAMPLES:
  export page abc123                              Export single page as markdown
  export page abc123 --format html --recursive    Export page tree as HTML
  export workspace --output my-backup.zip         Export entire workspace
  export poll task-abc --output backup.zip        Resume a timed-out export`
