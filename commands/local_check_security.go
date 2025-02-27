package commands

import (
	"fmt"
	"os"

	"github.com/fabpot/local-php-security-checker/security"
	"github.com/pkg/errors"
	"github.com/symfony-cli/console"
	"github.com/symfony-cli/terminal"
)

var localSecurityCheckCmd = &console.Command{
	Category: "local",
	Name:     "check:security",
	Aliases:  []*console.Alias{{Name: "security:check"}, {Name: "check:security"}, {Name: "local:security:check"}},
	Usage:    "Check security issues in project dependencies",
	Description: `Checks security issues in project dependencies. Without arguments, it looks
for a "composer.lock" file in the current directory. Pass it explicitly to check
a specific "composer.lock" file.`,
	Flags: []console.Flag{
		dirFlag,
		&console.StringFlag{
			Name:         "format",
			DefaultValue: "ansi",
			Usage:        "The output format (ansi, markdown, json, junit, or yaml)",
			Validator: func(ctx *console.Context, format string) error {
				if format != "" && format != "markdown" && format != "json" && format != "yaml" && format != "ansi" && format != "junit" {
					return errors.Errorf(`format "%s" does not exist (supported formats: markdown, ansi, json, junit, and yaml)`, format)
				}

				return nil
			},
		},
		&console.StringFlag{Name: "archive", DefaultValue: security.AdvisoryArchiveURL, Usage: "Advisory archive URL"},
		&console.BoolFlag{Name: "local", Usage: "Do not make HTTP calls (needs a valid cache file)"},
		&console.BoolFlag{Name: "no-dev", Usage: "Do not check packages listed under require-dev"},
		&console.BoolFlag{Name: "update-cache", Usage: "Update the cache (other flags are ignored)"},
	},
	Action: func(c *console.Context) error {
		format := c.String("format")
		path := c.String("dir")
		advisoryArchiveURL := c.String("archive")

		db, err := security.NewDB(c.Bool("local"), advisoryArchiveURL)
		if err != nil {
			return console.Exit(fmt.Sprintf("unable to load the advisory DB: %s", err), 127)
		}

		if c.Bool("update-cache") {
			if err := db.Load(advisoryArchiveURL); err != nil {
				return console.Exit(err.Error(), 127)
			}
			return nil
		}

		lockReader, err := security.LocateLock(path)
		if err != nil {
			return console.Exit(err.Error(), 127)
		}

		lock, err := security.NewLock(lockReader)
		if err != nil {
			return console.Exit(fmt.Sprintf("unable to load the lock file: %s", err), 127)
		}

		vulns := security.Analyze(lock, db, c.Bool("no-dev"))

		output, err := security.Format(vulns, format)
		if err != nil {
			return console.Exit(fmt.Sprintf("unable to output the results: %s", err), 127)
		}
		terminal.Stdout.Write(output)

		if os.Getenv("GITHUB_WORKSPACE") != "" {
			// Ran inside a Github action, export vulns
			output, _ := security.Format(vulns, "raw_json")
			terminal.Eprintf("::set-output name=vulns::%s", output)
		}

		if vulns.Count() > 0 {
			return console.Exit("", 1)
		}
		return nil
	},
}
