# Rook-Ceph Bucket Provisioning

## Proposed Feature

Enable Rook-Ceph Object users the ability to request buckets via an ObjectBucketClaim, similar
to requesting a Persistent Volume using a Persistent Volume Claim.

### Overview

Currently, the Rook-Ceph operator enables the dynamic creation of Ceph cluster, object stores, and users via Rook native Custom Resource Definitions.  This proposal introduces the next logical step in this design: the addition a controller for handling dynamic bucket creation by leveraging the [lib-bucket-provisioner library](https://github.com/yard-turkey/lib-bucket-provisioner).  This library defines two new CRDs: ObjectBucketClaim and ObjectBucket.  Additionally, the library wraps a Kubernetes to watch related API events.

## Glossary

- Admin: A Kubernetes cluster administrator with RBAC permissions to instantiate and control access to cluster storage. The admin  provisions object stores via CephObjectStore custom resources and creates StorageClasses to enable user access.  
- User: A Kubernetes cluster user with limited permissions, typically confined to CRUD operations on Kubernetes objects within a single namespace.  The user will request Ceph buckets by instantiating ObjectBucketClaims.
- ObjectBucketClaim: a namespaced, user facing Custom Resource Definition representing a request for a bucket.  OBCs reference a StorageClass to differentiate between provisioners, in the same manner as a PVC would.  Depending on the StorageClass, they bucket may be created dynamically or preexisting.  In the later case, new credentials are created and attached to the existing bucket.
- ObjectBucket: a non-namespace, admin facing CRD representing the fulfillment of a user request for a bucket, in the same manner as PV.  ObjectBuckets serve to provide discrete information about the bucket and object store provider and are not intended for Users.

## Goals

- Automate the create of new (greenfield) Ceph buckets via the Kubernetes API.
- Automate authentication and connection to pre-existing (brownfield) Ceph bucktets via the Kubernetes API.
- Enable cluster admin control over bucket creation and access via the Kubernetes API.  
- Restrict users from creating buckets with automation-generated access keys outside of the Kubernetes API (e.g. via `s3cmd` tool)

## Non-Goals

- Does not enable bucket creation for other object store providers within Rook.  Those may also be implemented with the lib-bucket-provisioner library.
- Does not include the Swift interface implementation by Ceph Object.
- Does not allow for connecting to object stores which are not managed by the Rook-Ceph operator

## Requirements

1. Users must have permissions to PUT/GET/DELETE objects in the requested bucket.
1. Users must have permissions to define the  policies and ACLs of objects in the requested bucket.
1. Users must be provided credential and connection information in a Pod-consumable resource.
1. Credential and Connection information should be protected from accidental deletion.
1. The bucket naming convention, provided by the bucket lib, creates a unique bucket name  by concatenating the OBC's namespace + name.

## Use Cases

### Assumptions

- The Rook-Ceph Operator is deployed in namespace `rook-ceph-system`
- A CephCluster CRD instance created in namespace `rook-ceph-system`
- An object store and the accompanying CephObjectStore instance exist

**Use Case: Expose a Ceph Object Store for Bucket Provisioning (Greenfield)**

_As an admin, I want to expose an existing Ceph object store to users so they can begin requesting buckets via the Kubernetes API._

1. The admin creates a [StorageClass](#storageclass) with the fields:
    ```
    provisioner: ceph.rook.io/object
    parameters:
      objectStore: OBJECT-STORE
      objectStoreNamespace: OBJECT-STORE-NAMESPACE
    ```
1. The admin configures RBAC roles to enable Get and List access on the StorageClass(es)

**Use Case: Provision a Bucket (Greenfield)**

_As a Kubernetes user, I want to leverage the Kubernetes API to create object store buckets. I expect to get back the bucket endpoint and credentials information in a Pod-consumable API resource(s)._

1. The user creates an OBC with a reference to the desired StorageClass.
1. The operator detects the new OBC instance.<br/>
    2.1. The `provisioner` field of the OBC's StorageClass is checked.  If it matches, continue to the next step. Else, the process ends.<br/>
1. The object store FQDN is derived from the object store's Service and the namespace.
1. A new object store user is created.
1. A new bucket is created with the new user's credentials, making it the bucket owner<br/>
    4.1. In the case of a bucket name collision, the error is logged, and the process returns.  (Event recording is not yet implemented)
1. An OB is created to represent the bucket, with an objectReference to the OBC and phase set to Bound.
1. The OBC is updated with an objectReference to the OB.
1. A ConfigMap containing endpoint information is created in the namespace of the OBC
1. A Secret containing credentials is created in the namespace of the OBC
1. An app Pod may then reference the Secret and the ConfigMap to begin accessing the bucket.

**Use Case: Enable Access to a Pre-Created Bucket in the Object Store (Brownfield)**

_As an admin, I want to expose an existing Ceph object store to users so they can begin requesting buckets via the Kubernetes API._

1. The admin creates a [StorageClass](#storageclass) as before with, appending the `bucketName` field.  This directs the controller to generate a bucket policy for this bucket and associate a dynamically created key pair with it.
    ```
    provisioner: ceph.rook.io/object
    parameters:
      objectStore: OBJECT-STORE
      objectStoreNamespace: OBJECT-STORE-NAMESPACE
      bucketName: BUCKET-NAME
    ````
1. The admin configures RBAC roles to enable Get and List access to the StorageClass(es)

**Use Case: Connect to an existing bucket (Brownfield)**

_As a Kubernetes user, I want to leverage the Kubernetes API to request access to a bucket in a Ceph object store. I expect to get back the bucket endpoint and credentials information in a Pod-consumable API resource(s)._

1. The user creates an OBC in their namespace with the StorageClassName set to that of one with the `bucketName` field defined.
1. The operator detects the new OBC.
1. The operator checks the `provisioner` field of the OBC's StorageClass.  If it matches, continue to the next step. Else, the process ends.
1. The operator checks that the `bucketName` field is defined.
1. The object store service name is derived from the object
1. An object store user is created.
1. A bucket policy is created with the user as the Principle.
1. The bucket policy is applied to the bucket.
1. An OB is created to represent the bucket, with an objectReference to the OBC and phase set to Bound.
1. The OBC is updated with an objectReference to the OB.
1. A ConfigMap containing endpoint information is created in the namespace of the OBC
1. A Secret containing credentials is created in the namespace of the OBC
1. An app Pod may then reference the Secret and the ConfigMap to begin accessing the bucket.

**Use Case: Delete an Object Bucket**

_As a Kubernetes user, I want to delete ObjectBucketClaim instances and cleanup generated API resources._

1. The user deletes the OBC
1. The OBC is marked for deletion and left in the foreground. Through owner references, the Secret and ConfigMap are marked for deletion as well.  Finalizers are used to stall garbage collection until bucket deletion succeeds.
1. The operator executes the Deletion method.  Depending on the StorageClass' `retainPolicy`, the bucket is either deleted or suspended.
1. The finalizers are removed from the Secret, ConfigMap, and ObjectBucket.  The Secret and ConfigMap are garbage collected and the OBC is deleted.
1. The operator deletes the OB.


**Use Case: List All Provisioned Buckets in a Cluster**

_As an admin, I want to quickly list all dynamically created buckets in a cluster and access their object store specific metadata._

- The admin can get an at-a-glance picture of buckets by listing all ObjectBuckets in the cluster.
- The admin can get metadata specific to the bucket and its object store by executing a `kubectl describe` on a particular ObjectBucket.

---

## Looking Forward

- The bucket lib does not enforce Resource Quotas because quotas are not yet supported for CRDs.
A [PR](https://github.com/kubernetes/kubernetes/pull/72384) exists for enabling quotas on CRDs.

- Custom Bucket and Object policies are not defined for OBCs.  Currently all user keys will have Object PUT/GET/DELETE and object policy setting permissions.  It would be useful to allow users to link secondary keys with a subset of these permissions to buckets.

---

## API Specifications

### OBC Custom Resource Definition

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: objectbucketclaims.objectbucket.io
spec:
  group: objectbucket.io
  versions:
    - name: v1alpha1
      served: true
      storage: true
  names:
    kind: ObjectBucketClaim
    listKind: ObjectBucketClaimList
    plural: objectbucketclaims
    singular: objectbucketclaim
    shortNames:
      - obc
      - obcs
  scope: Cluster
  subresources:
    status: {}
```

### OBC Custom Resource (User Defined)

```yaml
apiVersion: objectbucket.io/v1alpha1
kind: ObjectBucketClaim
metadata:
  name: NAME
  namespace: USER-NAMESPACE
spec:
  storageClassName: OBJECT-STORE-STORAGE-CLASS [1]
  bucketName: BUCKET-NAME [2]
  generateBucketName: PREFIX [3]
  additionalConfig: [4]
  objectBucketName: `OB-NAME` [5]
  status:
    phase: {"pending", "bound", "released", "failed"}  [6]  
```
1. `storageClassName` is used to target the desired Object Store.  Used by the operator to get the Object Store service URL.
1. `bucketName` is the desired name of the Ceph bucket.  Mutually exclusive with `generateBucketName`.  Ignored for brownfield cases.
1. `generateBucketName` is used to prefix a randomized string for the bucket name.  Required if `bucketName` is not defined.
1. `additionalConfig` is a map intended for extending the API in cases where extra data is required by the provisioner
1. `objectBucketName` is set by the operator once the OB is created.  User defined values are ignored.
1. `phase` 3 possible phases of bucket creation, mutually exclusive
    - `pending`: the operator is processing the request
    - `bound`: the operator finished processing the request and linked the OBC and OB
    - `released`: the OBC has been deleted, leaving the OB unclaimed
    - `failed`: the operator cannot fulfill the request

### OB Custom Resource Definition

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: objectbuckets.objectbucket.io
spec:
  group: objectbucket.io
  versions:
    - name: v1alpha1
      served: true
      storage: true
  names:
    kind: ObjectBucket
    listKind: ObjectBucketList
    plural: objectbuckets
    singular: objectbucket
    shortNames:
      - ob
      - obs
  scope: Cluster
  subresources:
    status: {}
```

### OB Custom Resource

```yaml
apiVersion: objectbucket.io/v1alpha1
kind: ObjectBucket
metadata:
  name: OB-NAME
  finalizers:
  - objectbucket.io/finalizer
spec:
  reclaimPolicy: {"Retain", "Delete"} [1]
  storageClassName: OBJECT-STORE-STORAGE-CLASS
  connection:
    additionalState:
      cephUser:  [2]
    endpoint:
      additionalConfig: [3]
      bucketHost: OBJECT-STORE-CLUSTER-FQDN [4]
      bucketName: BUCKET-NAME
      bucketPort: PORT
      region: "us-west-1"
      ssl: boolean
  claimRef: [5]
    apiVersion: objectbucket.io/v1alpha1
    kind: ObjectBucketClaim
    name: OBC-NAME
    namespace: OBC-NAMESPACE
    uid: 1200bbdd-9146-11e9-97d9-eea3d2fec0e3
status:
  phase: {"bound", "released", "failed"} [6]
```

1. `reclaimPolicy`
    - `Delete` indicates the bucket is to be deleted.  Ignored for brownfield buckets.
    - `Retain` indicates the bucket should be suspended.
1. `cephUser` is a ceph-provisioner set field to track the generated user
1. `additionalConfig` is a string:string map available for provisioner specific data
1. `bucketHost` is the object store's FQDN
1. `claimRef` is an objectReference to the ObjectBucketClaim associated with this ObjectBucket
1. `phase` is the current state of the ObjectBucket
    - `bound`: the operator finished processing the request and linked the OBC and OB
    - `released`: the OBC has been deleted. Deletion of the OB _should_ be imminent.
    - `failed`: the provisioner cannot fulfill an OBC1.

### (User Access Key) Secret

```yaml
apiVersion: objectbucket.io/v1alpha1
kind: Secret
metadata:
  name: OB-NAME [1]
  namespace: USER-NAMESPACE [2]
  ownerReferences:
  - name: OBC-NAME [3]
    ...
data:
  ACCESS_KEY_ID: ACCESS_KEY
  SECRET_ACCESS_KEY: SECRET_KEY
```

1. `name` is the OBC's name
1. `namespce` is that of a originating OBC
1. `ownerReference` makes this secret a child of the originating OBC

### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: CM-NAME [1]
  namespace: USER-NAMESPACE [2]
  ownerReferences: [3]
  - name: MY-BUCKET-1
    ...
data:
  S3_BUCKET_HOST: OBJECT-STORE-URL [4]
  S3_BUCKET_NAME: MY-BUCKET-1 [5]
  S3_BUCKET_PORT: 80 [6]
  S3_BUCKET_SSL: no [7]
  S3_BUCKET_REGION: us-west-1 [8]
```
1. `name` is the OBC's name
1. `namespace` is that of the originating OBC
1. `ownerReference` sets the ConfigMap as a child of the ObjectBucketClaim
1. `S3_BUCKET_HOST` is the in-cluster URL of the object store
1. `S3_BUCKET_PORT` is the port to connect through
1. `S3_BUCKET_NAME` is the bucket name
1. `S3_BUCKET_SSL` signals whether the connection is SSL protected
1. `S3_BUCKET_REGION` is the default Ceph S3 region

### StorageClass

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: SOME-OBJECT-STORE
provisioner: "ceph.rook.io/object-provisioner" [1]
parameters:
  objectStoreName: MY-STORE [2]
  objectStoreNamespace: MY-STORE-NAMESPACE [3]
  bucketName: BROWNFIELD-BUCKET [4]
```

1. `provisioner` the provisioner responsible to handling OBCs referencing this StorageClass
1. `objectStore` used by the operator to derive the object store Service name.
1. `objectStoreNamespace` the namespace of the object store
1. `bucketName` (brownfield only)  when set, tells the operator the request is for an existing bucket of that name.  Causes the operator to ignore the OBC's spec.generateBucketName and spec.bucketName fields.
