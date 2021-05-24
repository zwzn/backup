package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/logging"
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
		parts := strings.SplitN(u.Host, ".", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("host did not contain bucket and region")
		}

		bucket := parts[0]
		region := parts[1]
		host := parts[2]

		client := s3.New(s3.Options{
			Region: region,
			Credentials: aws.CredentialsProviderFunc(func(c context.Context) (aws.Credentials, error) {
				password, _ := u.User.Password()
				return aws.Credentials{
					AccessKeyID:     u.User.Username(),
					SecretAccessKey: password,
				}, nil
			}),
			EndpointResolver: s3.EndpointResolverFunc(func(region string, options s3.EndpointResolverOptions) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL: fmt.Sprintf("https://%s.%s", region, host),
				}, nil
			}),
			Logger: logging.NewStandardLogger(os.Stdout),
			ClientLogMode: aws.LogRetries |
				// aws.LogSigning |
				aws.LogRequest |
				// aws.LogRequestWithBody |
				aws.LogResponse |
				aws.LogResponseWithBody,
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

	bdata, err := ioutil.ReadAll(data)
	if err != nil {
		return err
	}

	log.Printf("Backing up %s", p)
	// contentType := strings.Split(http.DetectContentType(bdata), ";")[0]

	_, err = b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(p),
		Body:   bytes.NewReader(bdata),
		// ContentType: aws.String(contentType),
		// ContentLength: int64(len(bdata)),
		// ContentMD5:    aws.String(base64.StdEncoding.EncodeToString(md5.New().Sum(bdata))),

	})

	// o, err := b.client.ListObjects(ctx, &s3.ListObjectsInput{
	// 	Bucket: aws.String(b.bucket),
	// })
	os.Exit(1)

	return err
}

func (b *S3Backend) List(p string) ([]File, error) {
	panic("not implemented")

	ctx := context.Background()
	_, err := b.client.ListObjects(ctx, &s3.ListObjectsInput{
		Bucket: aws.String(b.bucket),
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *S3Backend) Read(p string) (File, error) {
	panic("not implemented")
	return nil, nil
}
