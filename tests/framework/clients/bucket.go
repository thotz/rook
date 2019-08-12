/*
Copyright 2016 The Rook Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clients

import (
	b64 "encoding/base64"
	"fmt"
	"strings"

	"github.com/rook/rook/tests/framework/installer"
	"github.com/rook/rook/tests/framework/utils"
)

// BucketOperation is a wrapper for rook bucket operations
type BucketOperation struct {
	k8sh      *utils.K8sHelper
	manifests installer.CephManifests
}

// CreateBucketOperation creates a new bucket client
func CreateBucketOperation(k8sh *utils.K8sHelper, manifests installer.CephManifests) *BucketOperation {
	return &BucketOperation{k8sh, manifests}
}

func (b *BucketOperation) CreateBucketStorageClass(namespace string, storeName string, storageClassName string, reclaimPolicy string, region string) error {
	return b.k8sh.ResourceOperation("create", b.manifests.GetBucketStorageClass(namespace, storeName, storageClassName, reclaimPolicy, region))
}

func (b *BucketOperation) DeleteBucketStorageClass(namespace string, storeName string, storageClassName string, reclaimPolicy string, region string) error {
	err := b.k8sh.ResourceOperation("delete", b.manifests.GetBucketStorageClass(namespace, storeName, storageClassName, reclaimPolicy, region))
	return err
}

func (b *BucketOperation) CreateObc(obcName string, storageClassName string, bucketName string, createBucket bool) error {
	return b.k8sh.ResourceOperation("create", b.manifests.GetObc(obcName, storageClassName, bucketName, createBucket))
}

func (b *BucketOperation) DeleteObc(obcName string, storageClassName string, bucketName string, createBucket bool) error {
	return b.k8sh.ResourceOperation("delete", b.manifests.GetObc(obcName, storageClassName, bucketName, createBucket))
}

// ObcExists Function to check that obc got created with secret and configmap
func (b *BucketOperation) ObcExists(obcName string) bool {
	var message string
	var err error
	//GetResource(blah) returns success if blah is or is not found.
	//err = success and found_sec not "No resources found." means it was found
	//err = success and found_sec contains "No resources found." means it was not found
	//err != success is an other error
	message, err = b.k8sh.GetResource("obc", obcName)
	if err == nil && !strings.Contains(message, "No resources found.") {
		logger.Infof("OB Exists")
	} else {
		logger.Infof("Unable to find OB")
		return false
	}
	message, err = b.k8sh.GetResource("secret", obcName)
	if err == nil && !strings.Contains(message, "No resources found.") {
		logger.Infof("Secret for OBC Exists")
	} else {
		logger.Infof("Unable to find secret for OBC")
		return false
	}
	message, err = b.k8sh.GetResource("cm", obcName)
	if err == nil && !strings.Contains(message, "No resources found.") {
		logger.Infof("Configmap for OBC Exists")
	} else {
		logger.Infof("Unable to find configmap for OBC")
		return false
	}
	return true
}

// Fetchs SecretKey, AccessKey for s3 client
func (b *BucketOperation) GetAccessKey(obcName string) (string, error) {
	args := []string{"get", "secret", obcName, "-o", "jsonpath={@.data.AWS_ACCESS_KEY_ID}"}
	AccessKey, err := b.k8sh.Kubectl(args...)
	if err != nil {
		return "", fmt.Errorf("Unable to find access key -- %s", err)
	}
	decode, _ := b64.StdEncoding.DecodeString(AccessKey)
	return string(decode), nil
}

func (b *BucketOperation) GetSecretKey(obcName string) (string, error) {
	args := []string{"get", "secret", obcName, "-o", "jsonpath={@.data.AWS_SECRET_ACCESS_KEY}"}
	SecretKey, err := b.k8sh.Kubectl(args...)
	if err != nil {
		return "", fmt.Errorf("Unable to find secret key-- %s", err)
	}
	decode, _ := b64.StdEncoding.DecodeString(SecretKey)
	return string(decode), nil

}
