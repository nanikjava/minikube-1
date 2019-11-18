package cmd



import (
	"github.com/spf13/cobra"
	"k8s.io/minikube/cmd/minikube/cmd/ui"
)

// cacheImageConfigKey is the config field name used to store which images we have previously cached

// addCacheCmd represents the cache add command
var cliCmd = &cobra.Command{
	Use:   "cli",
	Short: "Show minikube UI.",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Main()
	},
}

