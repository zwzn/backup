package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

	dataBytes, err := ioutil.ReadAll(data)
	if err != nil {
		return err
	}

	_, err = b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.path(p, t)),
		Body:   bytes.NewReader(dataBytes),
	})

	return err
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
