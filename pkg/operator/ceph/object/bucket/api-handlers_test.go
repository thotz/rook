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

package bucket

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/rook/rook/pkg/clusterd"
	testop "github.com/rook/rook/pkg/operator/test"
	exectest "github.com/rook/rook/pkg/util/exec/test"
	"k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestApiHandlers(t *testing.T) {
	clientset := testop.New(3)
	executor := &exectest.MockExecutor{
		MockExecuteCommandWithOutput: func(debug bool, actionName string, command string, args ...string) (string, error) {
			return `{"api-handlers":"api-handlers"}`, nil
		},
	}

	configDir, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(configDir)
	os.Setenv("POD_NAMESPACE", "test")
	defer os.Setenv("POD_NAMESPACE", "")
	context := &clusterd.Context{Clientset: clientset, Executor: executor, ConfigDir: configDir}
	provisioner := NewProvisioner(context, "test")
	reclaimpolicy := v1.PersistentVolumeReclaimDelete
	sc := storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ceph-delete-bucket",
		},
		Provisioner:   "ceph.rook.io/bucket",
		ReclaimPolicy: &reclaimpolicy,
		Parameters:    map[string]string{"objectStoreName": "test-store", "objectStoreNamespace": "test", "region": "us-east-1"},
	}
	clientset.StorageV1().StorageClasses().Create(&sc)
	provisioner.getStorageClassWithBackoff("test-ceph-delete-bucket")
	provisioner.getSecretWithBackoff("test", "secret")
}
