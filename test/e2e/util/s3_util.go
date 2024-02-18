/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

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
