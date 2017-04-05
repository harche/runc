// +build linux

package main

import (

	"fmt"
	"github.com/urfave/cli"
)


var execCommand = cli.Command{
	Name:  "exec",
	Usage: NAUsage,
	ArgsUsage: `Not Applicable`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "console",
			Usage:  NAUsage,
		},
		cli.StringFlag{
			Name:  "cwd",
			Usage: NAUsage,
		},
		cli.StringSliceFlag{
			Name:  "env, e",
			Usage: NAUsage,
		},
		cli.BoolFlag{
			Name:  "tty, t",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "user, u",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "process, p",
			Usage: NAUsage,
		},
		cli.BoolFlag{
			Name:  "detach,d",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "process-label",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "apparmor",
			Usage: NAUsage,
		},
		cli.BoolFlag{
			Name:  "no-new-privs",
			Usage: NAUsage,
		},
		cli.StringSliceFlag{
			Name:  "cap, c",
			Value: &cli.StringSlice{},
			Usage: NAUsage,
		},
		cli.BoolFlag{
			Name:   "no-subreaper",
			Usage:  NAUsage,
			Hidden: true,
		},
	},
	Action: func(context *cli.Context) error {
		return fmt.Errorf(NAUsage)
	},
	SkipArgReorder: true,
}

