package cmd

import "github.com/spf13/cobra"

func Root() *cobra.Command {
	r := &cobra.Command{
		Use:   "scintilla",
		Short: "Compile your Scintilla code into the bytecode",
	}
	return r
}
