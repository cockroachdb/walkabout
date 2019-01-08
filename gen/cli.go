// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.

// Package gen contains the implementation of the walkabout code generator.
package gen

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// buildID is set by a linker flag.
var buildID = "dev"

// Main is the entry point for the walkabout tool.  It is invoked from
// a main() method in the top-level walkabout package.
func Main() error {
	var config config
	rootCmd := &cobra.Command{
		Use: "walkabout",
		Short: `walkabout is a code-generation tool to enhance struct types.
https://github.com/cockroachdb/walkabout`,
		Example: `
walkabout InterfaceName 
  Generates support code to make all struct types that implement
  the given interface walkable.

walkabout --union UnionInterface ( InterfaceName | StructName ) ...
  Generates an interface called "UnionInterface" which will be
  implemented by the named struct types, or those structs that implement
  the named interface(s).

walkabout --union UnionInterface --reachable ( InterfaceName | StructName ) ...
  As above, but also includes all types in the same package that are
  transitively reachable from the named types.  This is useful for
  refitting an entire package where the existing types may not all
  share a common interface.
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			config.typeNames = args
			g, err := newGeneration(config)
			if err != nil {
				return err
			}
			return g.Execute()
		},
	}

	rootCmd.Flags().StringVarP(&config.dir, "dir", "d", ".",
		"the directory to operate in")

	rootCmd.Flags().StringVarP(&config.outFile, "out", "o", "",
		"overrides the output file name")

	rootCmd.Flags().BoolVarP(&config.reachable, "reachable", "r", false,
		`make all transitively reachable types in the same package also
implement the --union interface. Only valid when using --union.`)

	rootCmd.Flags().StringVarP(&config.union, "union", "u", "",
		`generate a new interface with the given name to be used as the
visitable interface.`)

	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "version",
			Short: "print version information",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Printf("walkabout version %s; %s", buildID, runtime.Version())
			},
		})

	return rootCmd.Execute()
}
