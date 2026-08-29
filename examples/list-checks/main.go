// Command list-checks reads the checks for a change's revision
// (GET /changes/{id}/revisions/{rev}/checks -> []CheckInfo, anonymous) and prints a
// colored, Web-UI-style summary using Gerrit's own palette -- the checks-plugin twin of
// the gerrit-sdk-go get-change-detail example. Values come from the generated
// checksclient models; only the formatting is hand-written.
//
// The Gerrit XSSI guard ()]}' prefix on every JSON body) is stripped by the gerritxssi
// transport borrowed verbatim from gerrit-sdk-go -- the guard is identical on plugin
// endpoints, so no plugin-specific handling is needed.
//
// Note on the list decode: the checks list endpoint is a SYNTHESIZED collection GET, and
// the emitted spec models its response as a skeleton {type:object}, so the generated
// Execute() returns an untyped map. We therefore decode the array into the generated
// CheckInfo model directly, over the same XSSI-stripping transport. Single-check GET
// (GetChangesChangeIdRevisionsRevisionIdChecksCheckId) is fully typed to *CheckInfo.
//
//	go run github.com/davido/gerrit-checks-sdk-go/v3/examples/list-checks@latest --change 623324
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	cc "github.com/davido/gerrit-checks-sdk-go/v3/checksclient"
	"github.com/davido/gerrit-checks-sdk-go/v3/gerritxssi"
)

func main() {
	url := flag.String("url", "https://gerrit-review.googlesource.com", "Gerrit base URL")
	change := flag.String("change", "623324", "numeric id or project~branch~Change-Id")
	revision := flag.String("revision", "current", "revision: 'current', a patch-set number, or a commit SHA")
	noColor := flag.Bool("no-color", false, "disable ANSI color")
	flag.Parse()
	useColor = computeColor(*noColor)

	base := strings.TrimRight(*url, "/")
	checks, err := listChecks(context.Background(), base, *change, *revision)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	printChecks(base, *change, *revision, checks)
}

// listChecks fetches the revision's checks over the XSSI-stripping transport and decodes
// them into the generated CheckInfo model. See the file header for why this is a manual
// decode rather than the generated Execute() (skeleton collection-list response).
func listChecks(ctx context.Context, base, change, revision string) ([]cc.CheckInfo, error) {
	url := fmt.Sprintf("%s/changes/%s/revisions/%s/checks/", base, change, revision)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := gerritxssi.Client().Do(req) // strip Gerrit's )]}' guard
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var checks []cc.CheckInfo
	if err := json.Unmarshal(body, &checks); err != nil {
		return nil, fmt.Errorf("decode checks: %w", err)
	}
	return checks, nil
}

// ---- presentation -------------------------------------------------------------

func printChecks(base, change, revision string, checks []cc.CheckInfo) {
	fmt.Println(rule())
	repo := ""
	if len(checks) > 0 {
		repo = checks[0].GetRepository()
	}
	header := fmt.Sprintf("Checks  change #%s", change)
	if len(checks) > 0 {
		header = fmt.Sprintf("Checks  change #%d  ·  patch set %d", checks[0].GetChangeNumber(), checks[0].GetPatchSetId())
	}
	fmt.Printf("  %s\n", sgr(header, bold))
	if repo != "" {
		fmt.Printf("  %s\n", fg(fmt.Sprintf("%s/c/%s/+/%s", base, repo, change), blue700))
	}
	fmt.Println(rule())

	section("Summary")
	fmt.Printf("    %s\n", summaryLine(checks))

	if len(checks) == 0 {
		fmt.Println(rule())
		return
	}

	section(fmt.Sprintf("Checks (revision %s)", revision))
	for _, c := range checks {
		name := c.GetCheckerName()
		if name == "" {
			name = c.GetCheckerUuid()
		}
		fmt.Printf("\n    %s  %s\n", stateChip(string(c.GetState())), sgr(name, bold))
		row("checker", link(c.GetCheckerUuid()))
		if m := strings.TrimSpace(c.GetMessage()); m != "" {
			row("message", firstLine(m))
		}
		if u := c.GetUrl(); u != "" {
			row("url", fg(u, blue700))
		}
		if b := c.GetBlocking(); len(b) > 0 {
			parts := make([]string, len(b))
			for i, cond := range b {
				parts[i] = fg(pascal(string(cond)), red600)
			}
			row("blocking", strings.Join(parts, ", "))
		}
		if t := timing(c); t != "" {
			row("timing", sgr(t, dim))
		}
	}
	fmt.Println(rule())
}

// summaryLine renders "N total  ( ✓ a successful, ✗ b failed, … )" with per-state color.
func summaryLine(checks []cc.CheckInfo) string {
	counts := map[string]int{}
	for _, c := range checks {
		counts[strings.ToUpper(string(c.GetState()))]++
	}
	if len(checks) == 0 {
		return sgr("no checks", dim)
	}
	// Stable, meaningful order.
	order := []string{"SUCCESSFUL", "FAILED", "RUNNING", "SCHEDULED", "NOT_STARTED", "NOT_RELEVANT"}
	var parts []string
	for _, s := range order {
		if counts[s] == 0 {
			continue
		}
		icon, color := stateAccent(s)
		parts = append(parts, fg(fmt.Sprintf("%s %d %s", icon, counts[s], strings.ToLower(pascal(s))), color))
	}
	return fmt.Sprintf("%s total  ( %s )", sgr(fmt.Sprintf("%d", len(checks)), bold), strings.Join(parts, ", "))
}

func timing(c cc.CheckInfo) string {
	switch {
	case c.GetStarted() != "" || c.GetFinished() != "":
		return fmt.Sprintf("started=%s finished=%s", dash(c.GetStarted()), dash(c.GetFinished()))
	case c.GetUpdated() != "":
		return fmt.Sprintf("updated=%s", c.GetUpdated())
	default:
		return ""
	}
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// ---- color / styling ----------------------------------------------------------
// Borrowed verbatim from gerrit-sdk-go's example (in turn from polygerrit-ui
// app-theme.ts). Zero-dependency ANSI, off when stdout is not a TTY, on NO_COLOR, or
// --no-color.

var useColor bool

const (
	bold = "1"
	dim  = "2"
)

type rgb struct{ r, g, b uint8 }

var (
	white     = rgb{255, 255, 255}
	black     = rgb{0, 0, 0}
	gray700   = rgb{95, 99, 104}
	yellow700 = rgb{242, 153, 0}
	green700  = rgb{24, 128, 56}
	green300  = rgb{129, 201, 149}
	red300    = rgb{242, 139, 130}
	red600    = rgb{217, 48, 37}
	blue700   = rgb{25, 103, 210}

	headerIndigo = rgb{62, 78, 138}
)

func computeColor(noColorFlag bool) bool {
	if noColorFlag || os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("CLICOLOR_FORCE") != "" {
		return true
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func sgr(s, code string) string {
	if !useColor {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func fg(s string, c rgb) string {
	if !useColor {
		return s
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", c.r, c.g, c.b, s)
}

func chip(s string, f, b rgb) string {
	if !useColor {
		return s
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm%s\x1b[0m", f.r, f.g, f.b, b.r, b.g, b.b, s)
}

func rule() string { return fg(strings.Repeat("─", 76), headerIndigo) }

func section(title string) {
	fmt.Println()
	fmt.Printf("  %s\n", sgr(strings.ToUpper(title), bold))
}

func row(label, value string) {
	fmt.Printf("    %s%s\n", sgr(fmt.Sprintf("%-9s", label), dim), value)
}

func link(s string) string { return fg(s, blue700) }

// stateChip renders a CheckState as a filled, colored badge -- green for success, red
// for failure, so a red/green scan of a change's checks is immediate.
func stateChip(state string) string {
	label := pascal(state)
	switch strings.ToUpper(state) {
	case "SUCCESSFUL":
		return chip(" "+label+" ", black, green300)
	case "FAILED":
		return chip(" "+label+" ", white, red600)
	case "RUNNING":
		return chip(" "+label+" ", black, yellow700)
	case "SCHEDULED":
		return chip(" "+label+" ", white, blue700)
	default: // NOT_STARTED, NOT_RELEVANT and any future states
		return chip(" "+label+" ", white, gray700)
	}
}

// stateAccent returns an (icon, color) for the compact summary line.
func stateAccent(state string) (string, rgb) {
	switch strings.ToUpper(state) {
	case "SUCCESSFUL":
		return "✓", green700
	case "FAILED":
		return "✗", red600
	case "RUNNING":
		return "●", yellow700
	case "SCHEDULED":
		return "○", blue700
	default:
		return "○", gray700
	}
}

// pascal turns an upper-snake enum value into PascalCase: NOT_STARTED -> NotStarted.
func pascal(s string) string {
	parts := strings.Split(strings.ToLower(s), "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
