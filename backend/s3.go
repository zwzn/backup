package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
)

type S3File struct {
	backend  *S3Backend
	name     string
	versions []time.Time
	isDir    bool
}

func (f *S3File) Name() string {
	return f.name
}

func (f *S3File) Versions() []time.Time {
	return f.versions
}

func (f *S3File) IsDir() bool {
	return f.isDir
}

func (f *S3File) Data(t time.Time) (io.ReadCloser, error) {
	return nil, nil
}

type S3Backend struct {
	root   string
	host   string
	bucket string
	client *s3.Client
}

func init() {
	Register("s3", func(u *url.URL) (Backend, error) {
		region := u.Query().Get("region")
		bucket := u.Query().Get("bucket")
		keyID := u.Query().Get("key-id")
		secretKey := u.Query().Get("secret-key")

		client := s3.New(s3.Options{
			Region: region,
			Credentials: aws.CredentialsProviderFunc(func(c context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     keyID,
					SecretAccessKey: secretKey,
				}, nil
			}),
			EndpointResolver: s3.EndpointResolverFunc(func(region string, options s3.EndpointResolverOptions) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL: fmt.Sprintf("https://%s", u.Host),
				}, nil
			}),
		})

		return &S3Backend{
			root:   u.Path,
			host:   u.Host,
			bucket: bucket,
			client: client,
		}, nil
	})
}

func (b *S3Backend) URI() string {
	return "s3://" + path.Join(b.bucket+"."+b.host, b.root)
}

func (b *S3Backend) path(p string, t time.Time) string {
	return path.Join(b.root, fmt.Sprintf("%s-%d.gz", p, t.Unix()))
}

func (b *S3Backend) Write(p string, t time.Time, data io.Reader) error {
	ctx := context.Background()

	fileSize := int64(0)

	if file, ok := data.(*os.File); ok {
		stat, err := file.Stat()
		if err == nil {
			fileSize = stat.Size()
		}
	}

	key := b.path(p, t)

	if fileSize < 1000 {
		dataBytes, err := ioutil.ReadAll(data)
		if err != nil {
			return err
		}

		_, err = b.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(b.bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(dataBytes),
		})
		return err
	}

	upload, err := b.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}

	readers, err := chunkReader(data, 10000)
	if err != nil {
		return errors.Wrap(err, "")
	}
	pl(len(readers), func(i int) error {
		_, err := b.client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(b.bucket),
			Key:        aws.String(key),
			PartNumber: int32(i),
			UploadId:   upload.UploadId,
			Body:       readers[i],
		})
		return err
	})
	_, err = b.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(b.bucket),
		Key:      aws.String(key),
		UploadId: upload.UploadId,
	})
	if err != nil {
		return errors.Wrap(err, "")
	}

	return nil
}

func pl(length int, callback func(i int) error) error {
	wg := &sync.WaitGroup{}
	errors := make([]error, length)
	for i := 0; i < length; i++ {
		wg.Add(1)
		// go func(i int) {
		errors[i] = callback(i)
		if errors[i] != nil {
			log.Print(errors[i])
		}
		wg.Done()
		// }(i)
	}
	wg.Wait()
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *S3Backend) List(p string) ([]File, error) {
	ctx := context.Background()
	objects, err := b.client.ListObjects(ctx, &s3.ListObjectsInput{
		Bucket: aws.String(b.bucket),
		Prefix: aws.String(path.Join(b.root, p)),
	})
	if err != nil {
		return nil, err
	}

	filesMap := map[string]*S3File{}

	for _, object := range objects.Contents {
		filePath, t := splitName(strings.Replace(*object.Key, b.root, "", 1))
		file, ok := filesMap[filePath]
		if ok {
			file.versions = append(file.versions, t)
		} else {
			filesMap[filePath] = &S3File{
				backend:  b,
				name:     filePath,
				versions: []time.Time{t},
				isDir:    false,
			}
		}
	}

	files := make([]File, 0, len(objects.Contents))
	for _, file := range filesMap {
		files = append(files, file)
	}
	return files, err
}

func (b *S3Backend) Read(p string) (File, error) {
	files, err := b.List(p)
	if err != nil {
		return nil, err
	}

	if len(files) != 1 {
		return nil, os.ErrNotExist
	}

	file := files[0]

	if file.Name() != p {
		return nil, os.ErrNotExist
	}

	return file, nil
}

func chunkReader(reader io.Reader, chunkSize int64) ([]io.Reader, error) {
	size, err := readerSize(reader)
	if err != nil {
		return nil, err
	}

	readers := make([]io.Reader, int(math.Ceil(float64(size)/float64(chunkSize))))
	go func() {
		for i := range readers {
			buffer := &bytes.Buffer{}
			io.CopyN(buffer, reader, chunkSize)
			readers[i] = buffer
		}
	}()
	return readers, nil
}

func readerSize(r io.Reader) (int64, error) {

	if r, ok := r.(*os.File); ok {
		stat, err := r.Stat()
		if err != nil {
			return 0, errors.Wrap(err, "unable to get file stat")
		}
		return stat.Size(), nil
	}

	return 0, fmt.Errorf("this reader does not support length")
}
