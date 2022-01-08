package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"

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
	Namespace    string `long:"namespace" short:"n" required:"true" description:"Namespace of the pod creating the connection."`
	PodName      string `long:"pod" required:"true" description:"Name of the pod creating the connection."`
	ToNamespace  string `long:"to-namespace" required:"true" description:"Namespace of the pod receiving the connection."`
	ToPodName    string `long:"to-pod" description:"Name of the pod receiving the connection."`
	ToExternalIP string `long:"to-ext-ip" description:"IP address identifying a host *outside* the kubernetes cluster the connection originates in."`
	ToProtocol   string `long:"to-protocol" choice:"udp" choice:"tcp" choice:"sctp" description:"Used when --to-ext-ip is specified, specify the protocol of the connection (udp, tcp, or sctp). Default to tcp."`
	ToPort       string `long:"to-port" description:"(Optional) Number or name of the port to connect to."`
}

func (c *EvalCommandOptions) Execute(args []string) error {

	err := requireOne(c, "ToPodName", "ToExternalIP")
	if err != nil {
		return err
	}
	if c.ToExternalIP != "" {
		if c.ToProtocol == "" {
			fmt.Fprintln(os.Stderr, "No protocol specified so defaulting to TCP. Use --to-protocol to change.")
			c.ToProtocol = "tcp"
		}
		if c.ToPort == "" {
			return fmt.Errorf("--to-port is required when using --to-ext-ip")
		}
	}

	a, err := app.NewApp(globalOptions.KubeConfig)
	if err != nil {
		return fmt.Errorf("Fatal error: %s", err.Error())
	}

	v := app.NewConsoleView(len(globalOptions.Verbose))
	defer v.Flush()
	return a.CheckAccess(v, c.Namespace, c.PodName, c.ToNamespace, c.ToPodName, c.ToPort, c.ToExternalIP, c.ToProtocol)
}

func requireOne(obj any, fieldNames ...string) error {

	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	r := reflect.ValueOf(obj)
	c := reflect.Indirect(r)

	nonEmpty := []string{}
	empty := []string{}
	for _, name := range fieldNames {
		tField, ok := t.FieldByName(name)
		if !ok {
			return fmt.Errorf("unable to find the type of field %s on %+v %+v", name, obj, t)
		}
		longName := tField.Tag.Get("long")

		vField := c.FieldByName(name)
		if vField.IsZero() {
			empty = append(empty, longName)
		} else {
			nonEmpty = append(nonEmpty, longName)
		}
	}

	argList := func(names []string) string {
		withDashes := util.Map(names, func(n string) string { return "--" + n })
		return "[" + strings.Join(withDashes, ", ") + "]"
	}
	if len(nonEmpty) == 0 {
		return fmt.Errorf("Exactly one of %s is required, but none provided.", argList(empty))
	}
	if len(nonEmpty) > 1 {
		return fmt.Errorf("Exactly one of %s is required, but all of %s provided.", argList(append(empty, nonEmpty...)), argList(nonEmpty))
	}
	return nil
}

func requireExactlyOne(fieldNames string, inputs ...string) error {
	nonEmpty := util.Filter(inputs, func(input string) bool { return input != "" })
	if len(nonEmpty) == 0 {
		return fmt.Errorf("Please specify one of %s", fieldNames)
	}
	if len(nonEmpty) > 1 {
		return fmt.Errorf("Only one of %s may be specified, but found %s", fieldNames, strings.Join(inputs, ", "))
	}
	return nil
}

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
		// err from either the parser or the executed command
		os.Exit(1)
	}
}
