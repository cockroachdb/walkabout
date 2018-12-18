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

package gen

import (
	"errors"
	"github.com/spf13/cobra"
)

// Main is the entry point for the walkabout tool.  It is invoked from
// a main() method in the top-level walkabout package.
func Main() error {
	var dir string
	rootCmd := &cobra.Command{
		Use:     "walkabout",
		Short:   "walkabout generates a visitor pattern from golang structs implementing a named interface",
		Example: "walkabout InterfaceName",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("at least one interface name must be specified")
			}
			return newGeneration(dir, args).Execute()
		},
		SilenceUsage: true,
	}
	rootCmd.Flags().StringVarP(&dir, "dir", "d", ".", "the directory to operate in")
	return rootCmd.Execute()
}
