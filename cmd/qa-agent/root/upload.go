package root

import (
    "fmt"

    "github.com/spf13/cobra"
)

var (
    uploadRunID   string
    uploadForce   bool
)

var uploadCmd = &cobra.Command{
    Use:   "upload",
    Short: "Upload cached runs to the server",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Uploading runs (run_id=%s, force=%v) ...\n", uploadRunID, uploadForce)
        fmt.Println("TODO: implement uploader with presigned URLs + resume.")
        return nil
    },
}

func init() {
    uploadCmd.Flags().StringVar(&uploadRunID, "run-id", "", "specific run ID to upload (default: all pending)")
    uploadCmd.Flags().BoolVar(&uploadForce, "force", false, "force upload even if offline flag set")
}
