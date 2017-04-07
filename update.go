// +build linux

package main

import (
	"github.com/urfave/cli"
	"fmt"
)

var updateCommand = cli.Command{
	Name:      "update",
	Usage:     NAUsage,
	ArgsUsage: `<container-id>`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "resources, r",
			Value: "",
			Usage: NAUsage,
		},

		cli.IntFlag{
			Name:  "blkio-weight",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "cpu-period",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "cpu-quota",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "cpu-share",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "cpu-rt-period",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "cpu-rt-runtime",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "cpuset-cpus",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "cpuset-mems",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "kernel-memory",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "kernel-memory-tcp",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "memory",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "memory-reservation",
			Usage: NAUsage,
		},
		cli.StringFlag{
			Name:  "memory-swap",
			Usage: NAUsage,
		},
	},
	Action: func(context *cli.Context) error {
		return fmt.Errorf(NAUsage)
	},
}
