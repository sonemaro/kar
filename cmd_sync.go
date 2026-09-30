package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// driftError makes "sync" exit with code 2 when reconciliation found
// something to fix, so scripts and agents can branch on the exit code.
type driftError struct{ count int }

func (e *driftError) Error() string {
	return fmt.Sprintf("%d drift(s) found — run \"kar sync --fix\" to reconcile", e.count)
}

type drift struct {
	Idea    string `json:"idea"`
	Summary string `json:"summary"`
	Kind    string `json:"kind"` // "stage" or "missing-dates"
	Detail  string `json:"detail"`
	Fixable bool   `json:"fixable"`
}

// runSync reconciles the roadmap against the work project. For every idea
// it follows the "Relates" link into the work project and compares
// completion:
//
//	work issue Done + idea not Done -> stage drift (fixable with --fix)
//	idea Done + work issue not Done -> warning only
//	missing start/target dates      -> information
func runSync(sh *shared, fs *flagSet, args []string) error {
	fix := fs.Bool("fix", false, "apply safe fixes (transition drifted ideas to Done)")
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	c, err := sh.client()
	if err != nil {
		return err
	}
	ideas, err := c.search(
		fmt.Sprintf("project = %s AND issuetype = Idea ORDER BY key ASC", sh.cfg.RoadmapProject),
		100, []string{"summary", "status", "issuelinks",
			sh.cfg.FieldStart, sh.cfg.FieldTarget, sh.cfg.FieldRoadmapLane})
	if err != nil {
		return err
	}

	workPrefix := sh.cfg.WorkProject + "-"
	var out []drift
	var drifts int
	for _, idea := range ideas {
		f, err := idea.decode()
		if err != nil {
			return err
		}
		epicKey := linkedWorkIssue(idea, workPrefix)
		if epicKey == "" {
			continue
		}
		epicStatus, err := c.epicStatus(epicKey)
		if err != nil {
			return err
		}
		ideaDone := strings.EqualFold(f.Status.Name, "Done")
		switch {
		case epicStatus == "Done" && !ideaDone:
			drifts++
			d := drift{Idea: idea.Key, Summary: f.Summary, Kind: "stage", Fixable: true,
				Detail: fmt.Sprintf("%s is Done, idea is %q", epicKey, f.Status.Name)}
			if *fix {
				if err := c.transitionTo(idea.Key, "Done"); err != nil {
					return err
				}
				d.Detail += " — fixed"
			}
			out = append(out, d)
		case ideaDone && epicStatus != "Done":
			out = append(out, drift{Idea: idea.Key, Summary: f.Summary, Kind: "stage",
				Detail: fmt.Sprintf("idea is Done but %s is %q", epicKey, epicStatus)})
		}
		if !idea.hasField(sh.cfg.FieldStart) || !idea.hasField(sh.cfg.FieldTarget) {
			out = append(out, drift{Idea: idea.Key, Summary: f.Summary, Kind: "missing-dates",
				Detail: "Project start or Project target not set"})
		}
	}

	if sh.json {
		if err := json.NewEncoder(stdout).Encode(map[string]any{
			"ideas": len(ideas), "drifts": out, "count": drifts, "fixed": *fix,
		}); err != nil {
			return err
		}
	} else {
		for _, d := range out {
			tag := "info"
			if d.Kind == "stage" {
				tag = "DRIFT"
			}
			fmt.Printf("%-5s %-14s %-26s %s\n", tag, d.Idea, d.Summary, d.Detail)
		}
		fmt.Printf("%d idea(s) checked, %d stage drift(s)\n", len(ideas), drifts)
	}
	if drifts > 0 && !*fix {
		return &driftError{count: drifts}
	}
	return nil
}

func linkedWorkIssue(idea rawIssue, workPrefix string) string {
	f, err := idea.decode()
	if err != nil {
		return ""
	}
	for _, l := range f.IssueLinks {
		for _, side := range []string{l.InwardIssue.Key, l.OutwardIssue.Key} {
			if side != "" && strings.HasPrefix(side, workPrefix) {
				return side
			}
		}
	}
	return ""
}

func (c *client) epicStatus(key string) (string, error) {
	var out struct {
		Fields struct {
			Status statusField `json:"status"`
		} `json:"fields"`
	}
	if err := c.do("GET", "/rest/api/3/issue/"+key+"?fields=status", nil, &out); err != nil {
		return "", err
	}
	return out.Fields.Status.Name, nil
}
