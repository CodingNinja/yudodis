/*
Copyright © 2024 David Mann me@dmann.dev
*/
package remote

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const debounceTime = time.Second * 2

// s3Writer allows you to write files from a channel
type s3Writer struct {
	bucket   string
	prefix   string
	uploader *s3manager.Uploader

	t *time.Timer
}

func NewS3WriterWithSession(sess *session.Session, bucket string, prefix string) *s3Writer {
	uploader := s3manager.NewUploader(sess)
	return &s3Writer{
		bucket:   bucket,
		prefix:   prefix,
		uploader: uploader,
	}
}

func (s3w *s3Writer) updateLockTime() {
	if s3w.t != nil && !s3w.t.Stop() {
		<-s3w.t.C
	}

	s3w.t = time.AfterFunc(debounceTime, func() {
		s3w.t = nil
		if err := s3w.upload("__lock__timer__", strings.NewReader(time.Now().Format(time.ANSIC))); err != nil {
			panic(fmt.Sprintf("unable to write lock timer - %s", err.Error()))
		}
	})
}

// Start reading a channel and uploading file names from that chan to the S3 bucket
func (s3w *s3Writer) Start(c chan string) error {
	for p := range c {
		f, err := os.Open(p)
		if err != nil {
			return err
		}

		if err := s3w.upload(p, f); err != nil {
			return err
		}

		go s3w.updateLockTime()
	}

	return nil
}

func (s3w *s3Writer) upload(key string, body io.Reader) error {
	_, err := s3w.uploader.UploadWithContext(context.Background(), &s3manager.UploadInput{
		Bucket: &s3w.bucket,
		Key:    aws.String(path.Join(s3w.prefix, key)),
		Body:   body,
	})

	return err
}
