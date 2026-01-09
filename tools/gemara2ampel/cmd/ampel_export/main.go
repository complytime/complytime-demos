package main

import (
	"flag"
	"fmt"
	"os"

	"gemara2ampel/go/cmd/ampel_export/export"
)

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: ampel_export <path> [flags]")
		fmt.Println("")
		fmt.Println("Converts a Gemara Layer-3 policy to Ampel verification policy format.")
		fmt.Println("")
		fmt.Println("Flags:")
		fmt.Println("  -output string")
		fmt.Println("        Path to output file (default \"ampel-policy.json\")")
		fmt.Println("  -catalog string")
		fmt.Println("        Path to catalog file for enriching policy details")
		fmt.Println("  -scope-filters")
		fmt.Println("        Include scope-based CEL filters in tenets (default false)")
		fmt.Println("  -policyset")
		fmt.Println("        Generate a PolicySet with imports as external references (default false)")
		fmt.Println("  -policyset-name string")
		fmt.Println("        Name for the PolicySet (only used with -policyset)")
		fmt.Println("  -policyset-description string")
		fmt.Println("        Description for the PolicySet (only used with -policyset)")
		fmt.Println("  -policyset-version string")
		fmt.Println("        Version for the PolicySet (only used with -policyset)")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  # Generate a single Ampel policy")
		fmt.Println("  ampel_export policy.yaml -output my-policy.json -catalog catalog.yaml")
		fmt.Println("")
		fmt.Println("  # Generate a PolicySet with imported policies as references")
		fmt.Println("  ampel_export policy.yaml -policyset -output my-policyset.json")
		os.Exit(1)
	}

	path := args[0]
	subcommandArgs := args[1:]

	if err := export.Policy(path, subcommandArgs); err != nil {
		fmt.Printf("Error processing policy: %v\n", err)
		os.Exit(1)
	}
}
