package main

import (
	"encoding/json"
	"fmt"
)

// runDoctor verifies the three things every other command depends on:
// authentication, both projects, and the configured customfield ids.
func runDoctor(sh *shared, fs *flagSet, args []string) error {
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	c, err := sh.client()
	if err != nil {
		return err
	}
	name, err := c.myself()
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	fmt.Printf("auth        ok (as %s on %s)\n", name, sh.cfg.Site)

	fail := false
	if err := c.project(sh.cfg.WorkProject); err != nil {
		fmt.Printf("work        FAIL (%s)\n", err)
		fail = true
	} else {
		fmt.Printf("work        ok (%s)\n", sh.cfg.WorkProject)
	}
	if err := c.project(sh.cfg.RoadmapProject); err != nil {
		fmt.Printf("roadmap     FAIL (%s)\n", err)
		fail = true
	} else {
		fmt.Printf("roadmap     ok (%s)\n", sh.cfg.RoadmapProject)
	}

	for _, f := range []struct {
		label, id string
	}{
		{"field start", sh.cfg.FieldStart},
		{"field target", sh.cfg.FieldTarget},
		{"field lane", sh.cfg.FieldRoadmapLane},
	} {
		ok, err := c.fieldExists(f.id)
		if err != nil {
			fmt.Printf("%-11s FAIL (%v)\n", f.label, err)
			fail = true
			continue
		}
		if !ok {
			fmt.Printf("%-11s FAIL (%s not found on this site)\n", f.label, f.id)
			fail = true
			continue
		}
		fmt.Printf("%-11s ok (%s)\n", f.label, f.id)
	}

	if sh.json {
		return json.NewEncoder(stdout).Encode(map[string]any{"ok": !fail, "user": name})
	}
	if fail {
		return fmt.Errorf("doctor found problems")
	}
	return nil
}
