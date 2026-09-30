package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// resolveIdea accepts either an issue key (HAMROADMAP-11) or a summary
// fragment ("v0.4"); a fragment must match exactly one idea in the
// roadmap project to resolve.
func resolveIdea(c *client, project, arg string) (string, string, error) {
	if keyPattern.MatchString(strings.ToUpper(arg)) {
		return strings.ToUpper(arg), arg, nil
	}
	issues, err := c.search(
		fmt.Sprintf("project = %s AND summary ~ %q ORDER BY key ASC", project, arg),
		10, []string{"summary"})
	if err != nil {
		return "", "", err
	}
	switch len(issues) {
	case 1:
		f, err := issues[0].decode()
		if err != nil {
			return "", "", err
		}
		return issues[0].Key, f.Summary, nil
	case 0:
		return "", "", fmt.Errorf("no idea in %s matches %q", project, arg)
	default:
		var keys []string
		for _, i := range issues {
			f, err := i.decode()
			if err != nil {
				return "", "", err
			}
			keys = append(keys, i.Key+" "+f.Summary)
		}
		return "", "", fmt.Errorf("%q is ambiguous, matches: %s", arg, strings.Join(keys, "; "))
	}
}

// runStage moves an idea to a named status ("Parking lot", "Discovery",
// "Delivery", "Done" — whatever the roadmap workflow defines).
func runStage(sh *shared, fs *flagSet, args []string) error {
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf(`usage: kar stage <idea> <"Parking lot"|Discovery|Delivery|Done>`)
	}
	c, err := sh.client()
	if err != nil {
		return err
	}
	key, summary, err := resolveIdea(c, sh.cfg.RoadmapProject, fs.Arg(0))
	if err != nil {
		return err
	}
	stage := fs.Arg(1)
	if err := c.transitionTo(key, stage); err != nil {
		return err
	}
	if sh.json {
		return json.NewEncoder(stdout).Encode(map[string]string{"key": key, "summary": summary, "stage": stage})
	}
	fmt.Printf("%s (%s) -> %s\n", key, summary, stage)
	return nil
}

// runDates sets the Project start / Project target interval fields that
// Jira Product Discovery timelines draw bars from.
func runDates(sh *shared, fs *flagSet, args []string) error {
	start := fs.String("start", "", "Project start date, YYYY-MM-DD")
	target := fs.String("target", "", "Project target date, YYYY-MM-DD")
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 || (*start == "" && *target == "") {
		return fmt.Errorf("usage: kar dates <idea> [--start YYYY-MM-DD] [--target YYYY-MM-DD]")
	}
	if err := validDate(*start); err != nil {
		return err
	}
	if err := validDate(*target); err != nil {
		return err
	}
	c, err := sh.client()
	if err != nil {
		return err
	}
	key, summary, err := resolveIdea(c, sh.cfg.RoadmapProject, fs.Arg(0))
	if err != nil {
		return err
	}
	fields := map[string]any{}
	if *start != "" {
		fields[sh.cfg.FieldStart] = intervalValue(*start)
	}
	if *target != "" {
		fields[sh.cfg.FieldTarget] = intervalValue(*target)
	}
	if err := c.editFields(key, fields); err != nil {
		return err
	}
	if sh.json {
		return json.NewEncoder(stdout).Encode(map[string]string{
			"key": key, "summary": summary, "start": *start, "target": *target})
	}
	fmt.Printf("%s (%s) dates updated: start=%s target=%s\n", key, summary, orDash(*start), orDash(*target))
	return nil
}

// runLane sets the roadmap lane select field (Now / Next / Later / Won't do).
func runLane(sh *shared, fs *flagSet, args []string) error {
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("usage: kar lane <idea> <Now|Next|Later|Won't do>")
	}
	c, err := sh.client()
	if err != nil {
		return err
	}
	key, summary, err := resolveIdea(c, sh.cfg.RoadmapProject, fs.Arg(0))
	if err != nil {
		return err
	}
	lane := fs.Arg(1)
	if err := c.editFields(key, map[string]any{
		sh.cfg.FieldRoadmapLane: map[string]string{"value": lane},
	}); err != nil {
		return err
	}
	if sh.json {
		return json.NewEncoder(stdout).Encode(map[string]string{"key": key, "summary": summary, "lane": lane})
	}
	fmt.Printf("%s (%s) lane -> %s\n", key, summary, lane)
	return nil
}

func validDate(d string) error {
	if d == "" {
		return nil
	}
	if len(d) != 10 || d[4] != '-' || d[7] != '-' {
		return fmt.Errorf("date %q must be YYYY-MM-DD", d)
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
