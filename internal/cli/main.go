package cli

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func Main() {
	_ = godotenv.Load() // best-effort: load .env if present

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
	root.Flags().Int("clips", 12, "Number of clips")

	// Hidden tuning flag (internal)
	root.Flags().Int("max", 60, "Max clip duration seconds")
	_ = root.Flags().MarkHidden("max")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
