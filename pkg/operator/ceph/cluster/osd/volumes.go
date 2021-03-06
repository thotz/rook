/*
Copyright 2020 The Rook Authors. All rights reserved.

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

package osd

import (
	"fmt"
	"path"

	v1 "k8s.io/api/core/v1"
)

const (
	udevPath             = "/run/udev"
	udevVolName          = "run-udev"
	osdEncryptionVolName = "osd-encryption-key"
)

func getPvcOSDBridgeMount(claimName string) v1.VolumeMount {
	return v1.VolumeMount{Name: fmt.Sprintf("%s-bridge", claimName), MountPath: "/mnt"}
}

func getPvcOSDBridgeMountActivate(mountPath, claimName string) v1.VolumeMount {
	return v1.VolumeMount{Name: fmt.Sprintf("%s-bridge", claimName), MountPath: mountPath, SubPath: path.Base(mountPath)}
}

func getPvcMetadataOSDBridgeMount(claimName string) v1.VolumeMount {
	return v1.VolumeMount{Name: fmt.Sprintf("%s-bridge", claimName), MountPath: "/srv"}
}
func getPVCOSDVolumes(osdProps *osdProperties) []v1.Volume {
	volumes := []v1.Volume{
		{
			Name: osdProps.pvc.ClaimName,
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &osdProps.pvc,
			},
		},
		{
			// We need a bridge mount which is basically a common volume mount between the non privileged init container
			// and the privileged provision container or osd daemon container
			// The reason for this is mentioned in the comment for getPVCInitContainer() method
			Name: fmt.Sprintf("%s-bridge", osdProps.pvc.ClaimName),
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{
					Medium: "Memory",
				},
			},
		},
	}

	// If we have a metadata PVC let's add it
	if osdProps.onPVCWithMetadata() {
		metadataPVCVolume := []v1.Volume{
			{
				Name: osdProps.metadataPVC.ClaimName,
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &osdProps.metadataPVC,
				},
			},
			{
				// We need a bridge mount which is basically a common volume mount between the non privileged init container
				// and the privileged provision container or osd daemon container
				// The reason for this is mentioned in the comment for getPVCInitContainer() method
				Name: fmt.Sprintf("%s-bridge", osdProps.metadataPVC.ClaimName),
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{
						Medium: "Memory",
					},
				},
			},
		}

		volumes = append(volumes, metadataPVCVolume...)
	}

	logger.Debugf("volumes are %+v", volumes)

	return volumes
}

func getUdevVolume() (v1.Volume, v1.VolumeMount) {
	volume := v1.Volume{
		Name: udevVolName,
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{Path: udevPath},
		},
	}

	volumeMounts := v1.VolumeMount{
		Name:      udevVolName,
		MountPath: udevPath,
	}

	return volume, volumeMounts
}
