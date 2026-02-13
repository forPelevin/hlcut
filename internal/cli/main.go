package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func Main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "warning: couldn't load .env: %v\n", err)
	}

	root := &cobra.Command{
		Use:          "hlcut <input>",
		Short:        "Cut highlight clips from a local MP4",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args[0])
		},
	}

	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.SilenceErrors = true

	// Visible flags
	root.Flags().String("out", "out", "Output directory")
	root.Flags().Int("clips", 12, "Max clips to return")
	root.Flags().Bool("burn-subtitles", false, "Burn karaoke subtitles into clips and write ASS files")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
