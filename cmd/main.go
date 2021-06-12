package main

import (
	"log"
	"os"
	"sort"

	"github.com/heyfey/celeste/cmd/cmd"
	"github.com/heyfey/celeste/config"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = config.Name
	app.Version = config.Version
	app.Usage = "DLT jobs scheduler"
	app.Description = "Manage training jobs in Celeste scheduler"
	app.Commands = []*cli.Command{
		{
			Name:   "create",
			Usage:  "Create a new training job from YAML",
			Action: cmd.CreateJob,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "filename",
					Aliases:  []string{"f"},
					Usage:    "`FILENAME` to use to create the training job",
					Required: true,
				},
			},
		},
		{
			Name:   "delete",
			Usage:  "Delete a training job by name",
			Action: cmd.DeleteJob,
		},
		{
			Name:  "get",
			Usage: "Display one or many resources",
			Subcommands: []*cli.Command{
				{
					Name:   "jobs",
					Usage:  "Prints a table of all training jobs.",
					Action: cmd.GetJobs,
				},
			},
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
