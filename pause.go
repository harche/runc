// +build linux

package main

import "github.com/urfave/cli"

var pauseCommand = cli.Command{
	Name:  "pause",
	Usage: "pause suspends the virtual machine",
	ArgsUsage: "",
	Description: "The pause command suspends a running virtual machine.",
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}
		container, err := getContainer(context)
		if err != nil {
			return err
		}
		if err := container.Pause(); err != nil {
			return err
		}

		return nil
	},
}

var resumeCommand = cli.Command{
	Name:  "resume",
	Usage: "resumes the suspended virtual machine",
	ArgsUsage: "",
	Description: "The resume command resumes the suspended virtual machine",
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}
		container, err := getContainer(context)
		if err != nil {
			return err
		}
		if err := container.Resume(); err != nil {
			return err
		}

		return nil
	},
}
