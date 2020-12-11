package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"github.com/jrockway/kubectl-jq/pkg/cmd"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	versionString := fmt.Sprintf("kubectl-jq version %s (%s) built on %s by %s", version, commit, date, builtBy)
	flags := pflag.NewFlagSet("kubectl-jq", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := cmd.NewCmdJQ(
		genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		versionString)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
