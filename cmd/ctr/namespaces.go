package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/metadata"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var namespacesCommand = cli.Command{
	Name:  "namespaces",
	Usage: "manage namespaces",
	Subcommands: cli.Commands{
		namespacesCreateCommand,
		namespacesSetLabelsCommand,
		namespacesListCommand,
		namespacesRemoveCommand,
	},
}

var namespacesCreateCommand = cli.Command{
	Name:        "create",
	Usage:       "Create a new namespace.",
	ArgsUsage:   "[flags] <name> [<key>=<value]",
	Description: "Create a new namespace. It must be unique.",
	Action: func(clicontext *cli.Context) error {
		var (
			ctx               = context.Background()
			namespace, labels = namespaceWithLabelArgs(clicontext)
		)

		if namespace == "" {
			return errors.New("please specify a namespace")
		}

		namespaces, err := getNamespacesService(clicontext)
		if err != nil {
			return err
		}

		if err := namespaces.Create(ctx, namespace, labels); err != nil {
			return err
		}

		return nil
	},
}

func namespaceWithLabelArgs(clicontext *cli.Context) (string, map[string]string) {
	var (
		namespace    = clicontext.Args().First()
		labelStrings = clicontext.Args().Tail()
		labels       = make(map[string]string, len(labelStrings))
	)

	for _, label := range labelStrings {
		parts := strings.SplitN(label, "=", 2)
		key := parts[0]
		value := "true"
		if len(parts) > 1 {
			value = parts[1]
		}

		labels[key] = value
	}

	return namespace, labels
}

var namespacesSetLabelsCommand = cli.Command{
	Name:        "set-labels",
	Usage:       "Set and clear labels for a namespace.",
	ArgsUsage:   "[flags] <name> [<key>=<value>, ...]",
	Description: "Set and clear labels for a namespace.",
	Flags:       []cli.Flag{},
	Action: func(clicontext *cli.Context) error {
		var (
			ctx               = context.Background()
			namespace, labels = namespaceWithLabelArgs(clicontext)
		)

		namespaces, err := getNamespacesService(clicontext)
		if err != nil {
			return err
		}

		if namespace == "" {
			return errors.New("please specify a namespace")
		}

		for k, v := range labels {
			if err := namespaces.SetLabel(ctx, namespace, k, v); err != nil {
				return err
			}
		}

		return nil
	},
}

var namespacesListCommand = cli.Command{
	Name:        "list",
	Aliases:     []string{"ls"},
	Usage:       "List namespaces.",
	ArgsUsage:   "[flags]",
	Description: "List namespaces.",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "print only the namespace name.",
		},
	},
	Action: func(clicontext *cli.Context) error {
		var (
			ctx   = context.Background()
			quiet = clicontext.Bool("quiet")
		)

		namespaces, err := getNamespacesService(clicontext)
		if err != nil {
			return err
		}

		nss, err := namespaces.List(ctx)
		if err != nil {
			return err
		}

		if quiet {
			for _, ns := range nss {
				fmt.Println(ns)
			}
		} else {

			tw := tabwriter.NewWriter(os.Stdout, 1, 8, 1, ' ', 0)
			fmt.Fprintln(tw, "NAME\tLABELS\t")

			for _, ns := range nss {
				labels, err := namespaces.Labels(ctx, ns)
				if err != nil {
					return err
				}

				var labelStrings []string
				for k, v := range labels {
					labelStrings = append(labelStrings, strings.Join([]string{k, v}, "="))
				}
				sort.Strings(labelStrings)

				fmt.Fprintf(tw, "%v\t%v\t\n", ns, strings.Join(labelStrings, ","))
			}

			return tw.Flush()
		}

		return nil
	},
}

var namespacesRemoveCommand = cli.Command{
	Name:        "remove",
	Aliases:     []string{"rm"},
	Usage:       "Remove one or more namespaces",
	ArgsUsage:   "[flags] <name> [<name>, ...]",
	Description: "Remove one or more namespaces. For now, the namespace must be empty.",
	Action: func(clicontext *cli.Context) error {
		var (
			ctx     = context.Background()
			exitErr error
		)

		namespaces, err := getNamespacesService(clicontext)
		if err != nil {
			return err
		}

		for _, target := range clicontext.Args() {
			if err := namespaces.Delete(ctx, target); err != nil {
				if !metadata.IsNotFound(err) {
					if exitErr == nil {
						exitErr = errors.Wrapf(err, "unable to delete %v", target)
					}
					log.G(ctx).WithError(err).Errorf("unable to delete %v", target)
					continue
				}

			}

			fmt.Println(target)
		}

		return exitErr

	},
}
