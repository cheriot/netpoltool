package main

import (
	"fmt"
	"os"

	flags "github.com/jessevdk/go-flags"

	"github.com/cheriot/netpoltool/internal/app"
	"github.com/cheriot/netpoltool/internal/util"
)

var globalOptions ApplicationOptions

type ApplicationOptions struct {
	LogLevel   string `long:"log-level" hidden:"true" description:"Log level (trace, debug, info, warning, error, fatal, panic)."`
	KubeConfig string `long:"kubeconfig" description:"Absolute path to the kubeconfig file. Default to ~/.kube/config."`
	Verbose    []bool `short:"v" long:"verbose" description:"Show more detail on NetworkPolicy evaluation."`
}

type EvalCommandOptions struct {
	Namespace   string `long:"namespace" short:"n" required:"true" description:"Namespace of the pod creating the connection."`
	PodName     string `long:"pod" required:"true" description:"Name of the pod creating the connection."`
	ToNamespace string `long:"to-namespace" required:"true" description:"Namespace of the pod receiving the connection."`
	ToPodName   string `long:"to-pod" required:"true" description:"Name of the pod receiving the connection."`
	ToPort      string `long:"to-port" description:"(Optional) Number or name of the port to connect to."`
}

func (c *EvalCommandOptions) Execute(args []string) error {
	a, err := app.NewApp(globalOptions.KubeConfig)
	if err != nil {
		return fmt.Errorf("Fatal error: %s", err.Error())
	}

	v := app.NewConsoleView(len(globalOptions.Verbose))
	defer v.Flush()
	return a.CheckAccess(v, c.Namespace, c.PodName, c.ToNamespace, c.ToPodName, c.ToPort)
}

// npt eval -n foobar -p mypod --to-namespace bazbar --to-pod otherpod
// npt eval -n foobar -p mypod --to-namespace bazbar --to-labels label=value --to-ip=IP
// Accept deployment objects. Maybe services?

func main() {
	globalOptions = ApplicationOptions{}
	parser := flags.NewParser(&globalOptions, flags.Default)

	evalCmdDesc := "Given source and destination pods, evaluate if Network Policies allow the source pod to access any ports on the destination pod."
	_, err := parser.AddCommand("eval", evalCmdDesc, evalCmdDesc, &EvalCommandOptions{})
	if err != nil {
		panic(err.Error())
	}

	parser.CommandHandler = func(commander flags.Commander, args []string) error {
		util.Log.Tracef("AppOptions %+v", globalOptions)

		if globalOptions.LogLevel != "" {
			err = util.SetLogLevel(globalOptions.LogLevel)
			if err != nil {
				util.Log.Panicf("Invalid log level %s", globalOptions.LogLevel)
			}

		}

		return commander.Execute(args)
	}

	_, err = parser.Parse()
	if err != nil {
		// TODO err when no port is accessible
		os.Exit(1)
	}
}
