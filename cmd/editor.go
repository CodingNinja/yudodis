/*
Copyright © 2024 David Mann me@dmann.dev
*/
package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/codingninja/yudodis/remote"
	"github.com/codingninja/yudodis/signal"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

// editorCmd represents the editor command
var editorCmd = &cobra.Command{
	Use:   "editor",
	Short: "Watches files and uploads them to a bucket",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("missing watcher dir")
		}

		bucket, err := cmd.Flags().GetString("bucket")
		if err != nil {
			return fmt.Errorf("unable to get the bucket to use - %w", err)
		}
		prefix, err := cmd.Flags().GetString("prefix")
		if err != nil {
			return fmt.Errorf("unable to get the prefix path to use - %w", err)
		}

		watchDir := args[0]
		// Create new watcher.
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return fmt.Errorf("unable to create watcher - %w", err)
		}
		defer watcher.Close()

		sig := signal.CloseWatcher()

		closer := make(chan error)
		fileChangedChan := make(chan string)
		defer close(fileChangedChan)
		defer close(closer)

		sess, err := session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		})
		if err != nil {
			return fmt.Errorf("unable to create aws session - %w", err)
		}

		uploader := remote.NewS3WriterWithSession(sess, bucket, prefix)

		go func() {
			if err := uploader.Start(fileChangedChan); err != nil {
				closer <- err
			}
		}()

		// Start listening for events.
		go func() {
			for {
				select {
				case <-closer:
					return
				case _, ok := <-sig:
					if ok {
						closer <- nil
						return
					}
				case event, ok := <-watcher.Events:
					if !ok {
						closer <- errors.New("unable to get watcher event")
						return
					}

					if event.Has(fsnotify.Write) {
						log.Println("modified file:", event.Name)
						fileChangedChan <- event.Name
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						closer <- fmt.Errorf("got watcher error - %w", err)
						return
					}
				default:
					continue
				}
			}
		}()

		err = filepath.WalkDir(watchDir, func(s string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				err = watcher.Add(s)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("unable to watch files - %w", err)
		}

		log.Printf("waiting for new file modifications")

		// Block main goroutine forever.
		return <-closer
	},
}

func init() {
	rootCmd.AddCommand(editorCmd)
}
