package cmd

import (
	"fmt"
	"path/filepath"

	"os"

	"github.com/spf13/cobra"

	"tvshow/lang/bytecode"
	"tvshow/lang/lexer"
	"tvshow/lang/macro"
	"tvshow/lang/parser"
	"tvshow/lang/token"
)

func Build() *cobra.Command {
	b := &cobra.Command{
		Use:   "build",
		Short: "Build Scintilla code",
		RunE: func(cmd *cobra.Command, args []string) error {
			sourcePath, err := cmd.Flags().GetString("source")
			if err != nil {
				return err
			}
			destPath, err := cmd.Flags().GetString("destination")
			if err != nil {
				return err
			}
			includeDirs, err := cmd.Flags().GetStringSlice("include_dirs")
			if err != nil {
				return err
			}

			content, err := os.ReadFile(sourcePath)
			if err != nil {
				return fmt.Errorf("failed to read source file %s: %w", sourcePath, err)
			}

			l := lexer.New(sourcePath, string(content))
			tokens := l.Tokens()

			expander := macro.New()
			expander.Include = func(name string, pos token.Position) ([]token.Token, error) {
				for _, include := range includeDirs {
					file := filepath.Join(include, name)
					content, err := os.ReadFile(file)
					if err != nil {
						continue
					}
					return lexer.New(file, string(content)).Tokens(), nil
				}
				return nil, fmt.Errorf("cannot find %q in the following include dirs: %v", name, includeDirs)
			}
			expandedTokens, err := expander.Expand(tokens)
			if err != nil {
				return fmt.Errorf("macro expansion failed: %w", err)
			}

			ast, err := parser.Parse(expandedTokens)
			if err != nil {
				return fmt.Errorf("parse failed: %w", err)
			}

			prog, err := bytecode.Translate(ast)
			if err != nil {
				return fmt.Errorf("bytecode translation failed: %w", err)
			}

			data, err := prog.MarshalBinary()
			if err != nil {
				return fmt.Errorf("binary serialization failed: %w", err)
			}

			if err := os.WriteFile(destPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write output binary to %s: %w", destPath, err)
			}

			return nil
		},
	}
	b.Flags().String("destination", "a.scb", "where to write compiled bytecode")
	b.Flags().String("source", "main.sc", "your source file")
	b.Flags().StringSlice("include_dirs", []string{"include/"}, "your include directories")
	return b
}
