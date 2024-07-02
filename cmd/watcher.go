/*
Copyright © 2024 David Mann me@dmann.dev
*/
package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/spf13/cobra"
)

// watcherCmd represents the watcher command
var watcherCmd = &cobra.Command{
	Use:   "watcher",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		bucket, err := cmd.Flags().GetString("bucket")
		if err != nil {
			return fmt.Errorf("unable to get the bucket to use - %w", err)
		}
		prefix, err := cmd.Flags().GetString("prefix")
		if err != nil {
			return fmt.Errorf("unable to get the prefix path to use - %w", err)
		}
		baseDir, err := cmd.Flags().GetString("base")
		if err != nil {
			return fmt.Errorf("unable to get the base path to use - %w", err)
		}
		postUpdateCmd := ""
		if len(args) > 0 {
			postUpdateCmd = args[0]
		}

		log.Printf("base dir is %s\n", baseDir)

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		sess, err := session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		})
		if err != nil {
			return fmt.Errorf("unable to create aws session - %w", err)
		}

		s3Svc := s3.New(sess)
		downloader := s3manager.NewDownloader(sess)

		closer := make(chan error)
		defer close(closer)
		curLockTime := ""
		fetchFiles := make(chan string)

		go func() {
			for range ticker.C {
				o, e := s3Svc.GetObject(&s3.GetObjectInput{
					Bucket: aws.String(bucket),
					Key:    aws.String(path.Join(prefix, "__lock__timer__")),
				})
				if e != nil {
					closer <- e
					return
				}
				time, err := io.ReadAll(o.Body)
				if err != nil {
					closer <- err
					return
				}
				timeStr := string(time)
				if curLockTime != timeStr {
					curLockTime = timeStr
					fetchFiles <- timeStr
				}
			}
		}()

		go func() {
			for range fetchFiles {
				keys := make(chan string)
				go func() {
					defer close(keys)
					var marker *string
					for {
						objs, err := s3Svc.ListObjects(&s3.ListObjectsInput{
							Bucket: aws.String(bucket),
							Prefix: aws.String(prefix),
							Marker: marker,
						})
						if err != nil {
							closer <- err
							return
						}
						for _, obj := range objs.Contents {
							keys <- *obj.Key
						}
						if truncated := objs.IsTruncated; truncated == nil || !*truncated {
							return
						}
						marker = objs.NextMarker
					}
				}()

				for key := range keys {
					filePath := path.Join(baseDir, strings.ReplaceAll(key, prefix+"/", ""))
					if err := os.MkdirAll(path.Dir(filePath), 0o750); err != nil {
						closer <- err
						return
					}
					f, err := os.Create(filePath)
					if err != nil {
						closer <- err
						return
					}
					_, err = downloader.Download(f, &s3.GetObjectInput{
						Bucket: aws.String(bucket),
						Key:    aws.String(key),
					})
					f.Close()
					if err != nil {
						closer <- err
						return
					}
				}
				if len(postUpdateCmd) > 0 {
					cmd := exec.Command(postUpdateCmd)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					if err := cmd.Run(); err != nil {
						closer <- err
						return
					}
				}
			}
		}()

		return <-closer
	},
}

func init() {
	rootCmd.AddCommand(watcherCmd)

	base := ""
	cwd, err := os.Getwd()
	if err == nil {
		base = path.Join(cwd, "remote-files")
	}
	watcherCmd.Flags().StringP("base", "B", base, "Base path to output files")
}
