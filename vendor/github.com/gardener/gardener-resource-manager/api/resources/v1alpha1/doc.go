// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +k8s:deepcopy-gen=package
// +k8s:conversion-gen=github.com/gardener/gardener-resource-manager/api/resources
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta

//go:generate ../../../hack/update-codegen.sh

// Package v1alpha1 contains the configuration of the Gardener Resource Manager.
// +groupName=resources.gardener.cloud
package v1alpha1 // import "github.com/gardener/gardener-resource-manager/api/resources/v1alpha1"