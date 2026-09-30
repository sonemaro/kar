package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// shared holds the flags every subcommand accepts plus the resolved config.
type shared struct {
	configPath string
	site       string
	email      string
	token      string
	json       bool
	rps        int

	// resolved from config/flags by sh.client()
	cfg config
}

func newShared(name string) (*shared, *flag.FlagSet) {
	sh := &shared{}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.StringVar(&sh.configPath, "config", "", "config file path (default $KAR_CONFIG or ~/.config/kar/config.json)")
	fs.StringVar(&sh.site, "site", "", "Atlassian site base URL (overrides config and $KAR_SITE)")
	fs.StringVar(&sh.email, "email", "", "Atlassian account email (overrides config and $KAR_EMAIL)")
	fs.StringVar(&sh.token, "token", "", "API token (overrides config, $KAR_TOKEN, $JIRA_API_TOKEN)")
	fs.BoolVar(&sh.json, "json", false, "emit machine-readable JSON")
	fs.IntVar(&sh.rps, "rps", 4, "max REST requests per second")
	return sh, fs
}

// config is the on-disk shape (~/.config/kar/config.json). Everything is
// optional; environment variables and flags fill the gaps.
type config struct {
	Site             string `json:"site"`
	Email            string `json:"email"`
	TokenFile        string `json:"token_file"`
	WorkProject      string `json:"work_project"`
	RoadmapProject   string `json:"roadmap_project"`
	FieldStart       string `json:"field_project_start"`
	FieldTarget      string `json:"field_project_target"`
	FieldRoadmapLane string `json:"field_roadmap_lane"`
}

func defaultConfigPath() string {
	if p := os.Getenv("KAR_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "kar", "config.json")
}

func defaultTokenPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".jira-token")
}

// resolve merges sources: flags > environment > config file > defaults.
func (sh *shared) resolve() error {
	cfg := config{
		Site:             os.Getenv("KAR_SITE"),
		Email:            os.Getenv("KAR_EMAIL"),
		TokenFile:        defaultTokenPath(),
		FieldStart:       "customfield_10059",
		FieldTarget:      "customfield_10053",
		FieldRoadmapLane: "customfield_10054",
	}
	path := sh.configPath
	if path == "" {
		path = defaultConfigPath()
	}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &cfg); err != nil {
			return fmt.Errorf("config %s: %w", path, err)
		}
	}
	// token: flag > env > token file
	token := sh.token
	if token == "" {
		token = os.Getenv("KAR_TOKEN")
	}
	if token == "" {
		token = os.Getenv("JIRA_API_TOKEN")
	}
	if token == "" {
		tp := cfg.TokenFile
		if tp == "" {
			tp = defaultTokenPath()
		}
		if !strings.HasPrefix(tp, "~/") {
			if info, err := os.Stat(tp); err == nil && info.Mode().Perm() != 0o600 {
				fmt.Fprintf(os.Stderr, "kar: warning: %s is not mode 600\n", tp)
			}
		}
		b, err := os.ReadFile(tp)
		if err != nil {
			return errors.New("no API token: pass --token, set $KAR_TOKEN, or write the token to " + tp)
		}
		token = strings.TrimSpace(string(b))
	}
	if tp := cfg.TokenFile; strings.HasPrefix(tp, "~/") {
		home, _ := os.UserHomeDir()
		cfg.TokenFile = filepath.Join(home, tp[2:])
	}

	cfg.Site = firstNonEmpty(sh.site, cfg.Site)
	cfg.Email = firstNonEmpty(sh.email, cfg.Email)
	cfg.Site = strings.TrimRight(cfg.Site, "/")
	if cfg.Site == "" || cfg.Email == "" || token == "" {
		return errors.New("site, email and token are all required (--help for the precedence rules)")
	}
	cfg.WorkProject = firstNonEmpty(cfg.WorkProject, "HAMN")
	cfg.RoadmapProject = firstNonEmpty(cfg.RoadmapProject, "HAMROADMAP")

	sh.cfg = cfg
	sh.token = token
	return nil
}

func (sh *shared) client() (*client, error) {
	if err := sh.resolve(); err != nil {
		return nil, err
	}
	return newClient(sh.cfg.Site, sh.cfg.Email, sh.token, sh.rps), nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
