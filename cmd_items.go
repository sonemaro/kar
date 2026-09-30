package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var keyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*-\d+$`)

// runDone transitions an issue to Done, optionally adding a comment first.
func runDone(sh *shared, fs *flagSet, args []string) error {
	comment := fs.String("comment", "", "optional comment added before the transition")
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: kar done <key> [--comment text]")
	}
	c, err := sh.client()
	if err != nil {
		return err
	}
	key := strings.ToUpper(fs.Arg(0))
	if *comment != "" {
		if err := c.addComment(key, *comment); err != nil {
			return err
		}
	}
	if err := c.transitionTo(key, "Done"); err != nil {
		return err
	}
	if sh.json {
		return json.NewEncoder(stdout).Encode(map[string]string{"key": key, "status": "Done"})
	}
	fmt.Printf("%s -> Done\n", key)
	return nil
}

// runComment adds a plain-text comment.
func runComment(sh *shared, fs *flagSet, args []string) error {
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("usage: kar comment <key> <text>")
	}
	c, err := sh.client()
	if err != nil {
		return err
	}
	key := strings.ToUpper(fs.Arg(0))
	text := fs.Arg(1)
	if err := c.addComment(key, text); err != nil {
		return err
	}
	if sh.json {
		return json.NewEncoder(stdout).Encode(map[string]string{"key": key, "commented": "true"})
	}
	fmt.Printf("commented on %s\n", key)
	return nil
}

// runLink creates a "Relates" link between two issues.
func runLink(sh *shared, fs *flagSet, args []string) error {
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("usage: kar link <keyA> <keyB>")
	}
	c, err := sh.client()
	if err != nil {
		return err
	}
	a := strings.ToUpper(fs.Arg(0))
	b := strings.ToUpper(fs.Arg(1))
	if err := c.relate(a, b); err != nil {
		return err
	}
	if sh.json {
		return json.NewEncoder(stdout).Encode(map[string]string{"a": a, "b": b, "type": "Relates"})
	}
	fmt.Printf("%s Relates %s\n", a, b)
	return nil
}

// runFind searches issues by summary text via JQL.
func runFind(sh *shared, fs *flagSet, args []string) error {
	project := fs.String("project", "", "project key (default: work project)")
	limit := fs.Int("limit", 20, "max results")
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: kar find <query> [--project KEY]")
	}
	c, err := sh.client()
	if err != nil {
		return err
	}
	p := *project
	if p == "" {
		p = sh.cfg.WorkProject
	}
	q := fmt.Sprintf("project = %s AND summary ~ %q ORDER BY key ASC", p, fs.Arg(0))
	issues, err := c.search(q, *limit, []string{"summary", "status"})
	if err != nil {
		return err
	}
	if sh.json {
		type row struct {
			Key, Status, Summary string
		}
		rows := make([]row, 0, len(issues))
		for _, i := range issues {
			f, err := i.decode()
			if err != nil {
				return err
			}
			rows = append(rows, row{i.Key, f.Status.Name, f.Summary})
		}
		return json.NewEncoder(stdout).Encode(rows)
	}
	for _, i := range issues {
		f, err := i.decode()
		if err != nil {
			return err
		}
		fmt.Printf("%-14s %-12s %s\n", i.Key, f.Status.Name, f.Summary)
	}
	fmt.Printf("%d result(s)\n", len(issues))
	return nil
}
