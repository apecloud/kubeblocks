package util

import (
	_ "io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func UploadToS3(fileName, s3Directory, bucketName string) error {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("cn-northwest-1"),
	})
	if err != nil {
		log.Fatal("Error creating AWS session:", err)
		return err
	}
	svc := s3.New(sess)
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
		return err
	}
	name := strings.Split(fileName, "/")
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(filepath.Join(s3Directory, name[len(name)-1])),
		Body:   file,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("file uploaded successfully")
	return err
}
