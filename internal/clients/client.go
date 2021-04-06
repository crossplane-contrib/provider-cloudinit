/*
Copyright 2020 The Crossplane Authors.

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
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/crossplane-contrib/provider-cloudinit/internal/clients/cloudinit"
)

// NewCloudInitClient returns a cloudinit client
func NewCloudInitClient(useGzipCompression bool, useBase64Encoding bool, base64Boundary string) *cloudinit.Client {
	return cloudinit.NewClient(useGzipCompression, useBase64Encoding, base64Boundary)
}

// IsErrorNotFound is true when the error is a Kubernetes Not Found error
func IsErrorNotFound(err error) bool {
	return kerrors.IsNotFound(errors.Cause(err))
}
