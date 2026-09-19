package cmd

import "github.com/spf13/cobra"

func Build() *cobra.Command {
	b := &cobra.Command{
		Use:   "build",
		Short: "Build Scintilla code",
	}
	b.Flags().String("destination", "a.scb", "where to write compiled bytecode")
	b.Flags().String("source", "main.scb", "your source file")
	return b
}
