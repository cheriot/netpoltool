package main

import (
	"strings"

	"bytes"
	"fmt"
	"os"

	"k8s.io/cli-runtime/pkg/printers"

	"github.com/cheriot/netpoltool/internal/k8s/builders"

	"k8s.io/apimachinery/pkg/runtime"
)

func main() {

	// Create a directory per namespace and output all objects in that ns to their own file.
	if len(os.Args) != 2 || os.Args[1] == "" {
		fmt.Fprintf(os.Stderr, "Please provide one positional arg specifying the output directory.\n")
		os.Exit(1)
	}
	dir := os.Args[1]

	// An annoying amount of ceremony to open a directory
	file, err := os.Open(dir)
	defer file.Close()
	errorExit("open "+dir, err)

	fileInfo, err := file.Stat()
	errorExit("file.Stat()", err)
	if !fileInfo.IsDir() {
		fmt.Fprintf(os.Stderr, "The specified path must be a directory.")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Generating output in %s\n", file.Name())

	err = os.Chdir(file.Name())
	errorExit("move to root dir", err)

	for _, nsb := range builders.Namespaces {
		err = os.Mkdir(nsb.Name, 0755)
		errorExit("create ns dir", err)
		err = os.Chdir(nsb.Name)
		errorExit("move to ns dir", err)

		bs, err := printK8sObj(&nsb.Namespace)
		errorExit("write ns file", err)
		err = os.WriteFile("namespace.yaml", bs, 0644)
		errorExit("writing ns file"+nsb.Name, err)

		for _, child := range nsb.Objects {
			kind, name, childObj := child.Build()

			bs, err := printK8sObj(childObj)
			errorExit("printK8sObj "+name, err)

			fileName := strings.ToLower(fmt.Sprintf("%s-%s.yaml", kind, name))
			os.WriteFile(fileName, bs, 0644)
		}

		err = os.Chdir("..")
		errorExit("chdir out of ns dir", err)
	}
}

func errorExit(msg string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error generating test resources when %s: %s", msg, err.Error())
		os.Exit(1)
	}
}

func printK8sObj(obj runtime.Object) ([]byte, error) {
	// https://medium.com/@harshjniitr/reading-and-writing-k8s-resource-as-yaml-in-golang-81dc8c7ea800
	b := &bytes.Buffer{}
	y := printers.YAMLPrinter{}
	err := y.PrintObj(obj, b)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
