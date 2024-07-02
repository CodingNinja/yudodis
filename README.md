![Y U DO DIS](./doc/yudodis.jpg)

# Y U DO DIS??

Work around restricted environments by using S3 buckets as a proxy

## Run on your local editor
Will watch for file changes in the directory specified and upload to s3

    go run main.go editor . --bucket $bucket_name


## Run on the restricted environment

Will watch for updates to the files in S3 and sync them to the local FS prior to invoking test.sh on change

    go run main.go watcher  --bucket daves-transfer-bucket $PWD/tesh.sh
