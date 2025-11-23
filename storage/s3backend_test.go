package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func Test(t *testing.T) {
	endpoint := "localhost:9000"
	// accessKeyID := "minioadmin"
	// secretAccessKey := "minioadmin"
	useSSL := false

	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, &minio.Options{
		// Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Creds:  credentials.NewFileAWSCredentials("/Users/roland/development/vsix/_local/credentials", "default"),
		Secure: useSSL,
	})
	if err != nil {
		t.Error(err)
	}

	inf, err := minioClient.ListBuckets(context.TODO())
	if err != nil {
		t.Error(err)
	}
	for _, i := range inf {
		fmt.Println("Bucket name:", i.Name)
		fmt.Println("Bucket region:", i.BucketRegion)
		fmt.Println("Bucket creation date:", i.CreationDate)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for obj := range minioClient.ListObjects(ctx, "exts", minio.ListObjectsOptions{Recursive: true, MaxKeys: 1}) {
		fmt.Println("File:", obj.Key)
		fmt.Println("Err:", obj.Err)
		// cancel()
	}
}

// TODO remove
// func TestFromPath(t *testing.T) {
// 	bkt, p := fromPath("exts/publisher/extension")
// 	if bkt != "exts" {
// 		t.Errorf("expected bucket to be %v but got %v", "exts", bkt)
// 	}
// 	if p != "publisher/extension" {
// 		t.Errorf("expected path to be %v but got %v", "publisher/extension", p)
// 	}
// }
