package S3Sink

import (
	"fmt"
	"sync"

	"github.com/chrislusf/seaweedfs/weed/pb/filer_pb"
	"github.com/chrislusf/seaweedfs/weed/replication/source"
	"github.com/chrislusf/seaweedfs/weed/util"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/chrislusf/seaweedfs/weed/filer2"
	"github.com/chrislusf/seaweedfs/weed/replication/sink"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

type S3Sink struct {
	conn        s3iface.S3API
	region      string
	bucket      string
	dir         string
	filerSource *source.FilerSource
}

func init() {
	sink.Sinks = append(sink.Sinks, &S3Sink{})
}

func (s3sink *S3Sink) GetName() string {
	return "s3"
}

func (s3sink *S3Sink) GetSinkToDirectory() string {
	return s3sink.dir
}

func (s3sink *S3Sink) Initialize(configuration util.Configuration) error {
	return s3sink.initialize(
		configuration.GetString("aws_access_key_id"),
		configuration.GetString("aws_secret_access_key"),
		configuration.GetString("region"),
		configuration.GetString("bucket"),
		configuration.GetString("directory"),
	)
}

func (s3sink *S3Sink) SetSourceFiler(s *source.FilerSource) {
	s3sink.filerSource = s
}

func (s3sink *S3Sink) initialize(awsAccessKeyId, aswSecretAccessKey, region, bucket, dir string) (error) {
	s3sink.region = region
	s3sink.bucket = bucket
	s3sink.dir = dir

	config := &aws.Config{
		Region: aws.String(s3sink.region),
	}
	if awsAccessKeyId != "" && aswSecretAccessKey != "" {
		config.Credentials = credentials.NewStaticCredentials(awsAccessKeyId, aswSecretAccessKey, "")
	}

	sess, err := session.NewSession(config)
	if err != nil {
		return fmt.Errorf("create aws session: %v", err)
	}
	s3sink.conn = s3.New(sess)

	return nil
}

func (s3sink *S3Sink) DeleteEntry(key string, isDirectory, deleteIncludeChunks bool) error {

	if isDirectory {
		key = key + "/"
	}

	return s3sink.deleteObject(key)

}

func (s3sink *S3Sink) CreateEntry(key string, entry *filer_pb.Entry) error {

	if entry.IsDirectory {
		return nil
	}

	uploadId, err := s3sink.createMultipartUpload(key, entry)
	if err != nil {
		return err
	}

	totalSize := filer2.TotalSize(entry.Chunks)
	chunkViews := filer2.ViewFromChunks(entry.Chunks, 0, int(totalSize))

	var parts []*s3.CompletedPart
	var wg sync.WaitGroup
	for chunkIndex, chunk := range chunkViews {
		partId := chunkIndex + 1
		wg.Add(1)
		go func(chunk *filer2.ChunkView) {
			defer wg.Done()
			if part, uploadErr := s3sink.uploadPart(key, uploadId, partId, chunk); uploadErr != nil {
				err = uploadErr
			} else {
				parts = append(parts, part)
			}
		}(chunk)
	}
	wg.Wait()

	if err != nil {
		s3sink.abortMultipartUpload(key, uploadId)
		return err
	}

	return s3sink.completeMultipartUpload(key, uploadId, parts)

}

func (s3sink *S3Sink) UpdateEntry(key string, oldEntry, newEntry *filer_pb.Entry, deleteIncludeChunks bool) (foundExistingEntry bool, err error) {
	// TODO improve efficiency
	return false, nil
}