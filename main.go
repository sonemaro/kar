// Command kar (کار) keeps Atlassian Jira work items and Jira Product
// Discovery roadmaps up to date from the terminal.
//
// It speaks only the documented Jira Cloud REST API (v3) with a user API
// token, throttles itself, and prints stable output for humans (the
// default) and machines (--json).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// flagSet is an alias so command signatures read cleanly; stdout is a
// variable so tests can capture output.
type flagSet = flag.FlagSet

var stdout = os.Stdout

// parseArgs lets flags appear anywhere: "kar done KEY --comment x" parses
// the same as "kar done --comment x KEY". stdlib flag stops at the first
// positional, so we pre-sort flag tokens (with their values) in front.
func parseArgs(fs *flagSet, args []string) error {
	boolFlags := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) {
		boolFlags[f.Name] = f.DefValue == "true" || f.DefValue == "false"
	})
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if !strings.Contains(name, "=") && !boolFlags[name] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	return fs.Parse(append(flags, pos...))
}

const usage = `kar — کار — keep Jira work items and Jira Product Discovery roadmaps up to date.

Usage:
  kar <command> [flags]

Work items (the "work project", e.g. a software project):
  kar done    <key> [--comment text]        Transition an issue to Done, optionally comment first.
  kar comment <key> <text>                  Add a comment (plain text; blank line = new paragraph).
  kar link    <keyA> <keyB>                 Create a "Relates" link between two issues.

Roadmap ideas (the "roadmap project", e.g. Jira Product Discovery):
  kar stage <idea> <stage>                  Move an idea: "Parking lot", "Discovery", "Delivery", "Done".
  kar dates <idea> [--start D] [--target D] Set Project start / Project target dates (YYYY-MM-DD).
  kar lane  <idea> <Now|Next|Later|Won't do>  Set the roadmap lane field.

Both:
  kar find <query> [--project KEY] [--limit N]  Search issues by summary text.
  kar sync [--fix]                              Reconcile roadmap ideas against linked work issues.
  kar doctor                                    Check auth, projects and configured fields.

Shared flags (on any subcommand):
  --config PATH   config file (default ~/.config/kar/config.json, or $KAR_CONFIG)
  --site URL      Atlassian site base URL   (overrides config / $KAR_SITE)
  --email ADDR    Atlassian account email   (overrides config / $KAR_EMAIL)
  --token VALUE   API token                 (overrides config / $KAR_TOKEN / $JIRA_API_TOKEN)
  --json          emit machine-readable JSON
  --rps N         max REST requests per second (default 4)

Exit codes: 0 success · 1 error · 2 drift reported by "sync" without --fix.

kar uses only the documented Jira Cloud REST API v3 with your own API
token. Not affiliated with Atlassian; see LICENSE and the README.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}
	cmd, rest := os.Args[1], os.Args[2:]

	sh, fs := newShared(cmd)
	var err error
	switch cmd {
	case "doctor":
		err = runDoctor(sh, fs, rest)
	case "find":
		err = runFind(sh, fs, rest)
	case "sync":
		err = runSync(sh, fs, rest)
	case "done":
		err = runDone(sh, fs, rest)
	case "comment":
		err = runComment(sh, fs, rest)
	case "link":
		err = runLink(sh, fs, rest)
	case "stage":
		err = runStage(sh, fs, rest)
	case "dates":
		err = runDates(sh, fs, rest)
	case "lane":
		err = runLane(sh, fs, rest)
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "kar: unknown command %q\n\n%s", cmd, usage)
		os.Exit(1)
	}
	if err != nil {
		if derr, ok := err.(*driftError); ok {
			fmt.Fprintln(os.Stderr, "kar:", derr.Error())
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "kar:", err)
		os.Exit(1)
	}
}
