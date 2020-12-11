package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
)

const jqExample = `
    # print the ports that pods have configured
    kubectl jq pod .spec.containers[].ports[]
`

type JQOptions struct {
	genericclioptions.IOStreams
	configFlags *genericclioptions.ConfigFlags

	allNamespaces  bool
	ignoreNotFound bool
	outputFormat   string
	flatten        bool
	rawStrings     bool

	namespace    string
	resourceType string
	resource     string
	expr         string

	jq        *gojq.Query
	formatter Formatter
}

func NewJQOptions(streams genericclioptions.IOStreams) *JQOptions {
	return &JQOptions{
		configFlags:  genericclioptions.NewConfigFlags(true),
		IOStreams:    streams,
		flatten:      true,
		outputFormat: "jsonpretty",
	}
}

func NewCmdJQ(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewJQOptions(streams)

	cmd := &cobra.Command{
		Use:                   "jq [resource type] (resource name, blank for all) (jq expression, blank to just print)",
		Short:                 "Execute a JQ program against a resource and print the result",
		Example:               jqExample,
		DisableFlagsInUseLine: false,
		SilenceUsage:          true,
		Args:                  o.ValidateArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(c, args); err != nil {
				return fmt.Errorf("parse: %w", err)
			}
			if err := o.Run(); err != nil {
				return fmt.Errorf("run: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&o.allNamespaces, "all-namespaces", "A", o.allNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().BoolVar(&o.ignoreNotFound, "ignore-not-found", o.ignoreNotFound, "If the requested object does not exist the command will return exit code 0.")
	cmd.Flags().StringVarP(&o.outputFormat, "output", "o", o.outputFormat, outputHelp)
	cmd.Flags().BoolVar(&o.flatten, "flatten", o.flatten, "If true, execute the JQ program over each item rather than a v1.List containing all the items.")
	cmd.Flags().BoolVarP(&o.rawStrings, "raw", "r", o.rawStrings, "If true, output bare strings without quotes.")
	o.configFlags.AddFlags(cmd.Flags())
	return cmd
}

// ValidateArgs checks the validity of positional args.
func (o *JQOptions) ValidateArgs(c *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New(`at least 1 argument is required -- the API group to inspect (like "kubectl jq pods" to inspect pods)`)
	}
	if len(args) > 3 {
		return errors.New(`at most 3 arguments are allowed; the API group to inspect, the resource to select, and a JQ expression`)
	}
	return nil
}

// Complete copies positional arguments into the JQOptions config and initializes objects needed for
// Run.
func (o *JQOptions) Complete(c *cobra.Command, args []string) error {
	cfg := o.configFlags.ToRawKubeConfigLoader()
	ns, _, err := cfg.Namespace()
	if err != nil {
		return fmt.Errorf("extract namespace from context: %w", err)
	}
	o.namespace = ns
	o.expr = "."
	switch len(args) {
	case 1:
		o.resourceType = args[0]
	case 2:
		o.resourceType = args[0]
		o.expr = args[1]
	case 3:
		o.resourceType = args[0]
		o.resource = args[1]
		o.expr = args[2]
	default:
		return o.ValidateArgs(c, args)
	}

	jq, err := gojq.Parse(o.expr)
	if err != nil {
		return fmt.Errorf("parse jq program: %w", err)
	}
	o.jq = jq

	o.formatter = outputFormat(o.outputFormat)
	return nil
}

// Run fetches and processes resources as specified by the JQOptions.
func (o *JQOptions) Run() error {
	builder := resource.NewBuilder(o.configFlags)
	b := builder.
		Unstructured().
		RequestChunksOf(500).
		NamespaceParam(o.namespace).
		DefaultNamespace().
		AllNamespaces(o.allNamespaces).
		ContinueOnError().
		Latest()
	if o.flatten {
		b = b.Flatten()
	}
	if o.resource == "" {
		b = b.ResourceTypes(o.resourceType).SelectAllParam(true)
	} else {
		b = b.ResourceNames(o.resourceType, o.resource)
	}
	result := b.Do()
	if o.ignoreNotFound {
		result.IgnoreErrors(apierrors.IsNotFound)
	}
	if err := result.Err(); err != nil {
		// This error seems to read best without an annotation.
		return err
	}
	if err := result.Visit(func(i *resource.Info, err error) error {
		if err != nil {
			return err
		}
		fields := make(map[string]interface{})
		bytes, err := json.Marshal(i.Object)
		if err != nil {
			return fmt.Errorf("convert to json: %w", err)
		}
		if err := json.Unmarshal(bytes, &fields); err != nil {
			return fmt.Errorf("convert to map[string]interface{}: %w", err)
		}
		iter := o.jq.Run(fields)
		for {
			w, f, newline := o.IOStreams.Out, o.formatter, true
			v, ok := iter.Next()
			if !ok {
				break
			}
			switch x := v.(type) {
			case error:
				return fmt.Errorf("jq: object %s/%s: %w", i.Namespace, i.Name, x)
			case [2]interface{}:
				if s, ok := x[0].(string); ok {
					w = o.IOStreams.ErrOut
					f = compactJSON{}
					newline = false
					if s == "STDERR:" {
						v = x[1]
					}
				}
			}
			if v == nil {
				continue
			}
			var bytes []byte
			if s, ok := v.(string); o.rawStrings && ok {
				bytes = []byte(s)
			} else {
				var err error
				bytes, err = f.Marshal(v)
				if err != nil {
					return fmt.Errorf("format: %w", err)
				}
			}
			if _, err := w.Write(bytes); err != nil {
				return fmt.Errorf("write: %w", err)
			}
			if newline {
				if _, err := w.Write([]byte("\n")); err != nil {
					return fmt.Errorf("write newline: %w", err)
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}
