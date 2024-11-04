package file

import (
	"context"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Uploader struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
}

func New(Endpoint, AccessKeyId, SecretAccessKey string) *S3Uploader {
	return &S3Uploader{
		Endpoint:        Endpoint,
		AccessKeyID:     AccessKeyId,
		SecretAccessKey: SecretAccessKey,
	}
}

func (s *S3Uploader) InitClient() (*minio.Client, error) {
	return minio.New(s.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s.AccessKeyID, s.SecretAccessKey, ""),
		Secure: true,
	})
}

func (s *S3Uploader) Upload(ctx context.Context, filePath string) (int64, error) {
	client, err := s.InitClient()
	if err != nil {
		return 0, err
	}
	bucketName := "testback"         // todo
	objectName := "etcd-snapshot.db" // todo
	uploadInfo, err := client.FPutObject(ctx, bucketName, objectName, filePath, minio.PutObjectOptions{})
	if err != nil {
		return 0, err
	}
	return uploadInfo.Size, nil
}
