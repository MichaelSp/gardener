// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extension

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as 'severe'.
	DefaultSevereThreshold = 30 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of an Extension resource.
	DefaultTimeout = 3 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Interface contains references to an Extension deployer.
type Interface interface {
	// DeleteStaleResources deletes unused Extension resources from the shoot namespace in the seed.
	DeleteStaleResources(context.Context) error
	// WaitCleanupStaleResources waits until all unused Extension resources are cleaned up.
	WaitCleanupStaleResources(context.Context) error
	// Extensions returns the map of extensions where the key is the type and the value is an Extension structure.
	Extensions() map[string]Extension

	// DeployBeforeKubeAPIServer deploys extensions that should be handled before the kube-apiserver.
	DeployBeforeKubeAPIServer(context.Context) error
	// RestoreBeforeKubeAPIServer restores extensions that should be handled before the kube-apiserver.
	RestoreBeforeKubeAPIServer(context.Context, *gardencorev1beta1.ShootState) error
	// WaitBeforeKubeAPIServer waits until all extensions that should be handled before the kube-apiserver are deployed and report readiness.
	WaitBeforeKubeAPIServer(context.Context) error

	// DeployAfterKubeAPIServer deploys extensions that should be handled after the kube-apiserver.
	DeployAfterKubeAPIServer(ctx context.Context) error
	// RestoreAfterKubeAPIServer restores extensions that should be handled after the kube-apiserver.
	RestoreAfterKubeAPIServer(ctx context.Context, shootState *gardencorev1beta1.ShootState) error
	// WaitAfterKubeAPIServer waits until all extensions that should be handled after the kube-apiserver are deployed and report readiness.
	WaitAfterKubeAPIServer(ctx context.Context) error

	// DestroyBeforeKubeAPIServer deletes the extensions that should be handled before the kube-apiserver.
	DestroyBeforeKubeAPIServer(context.Context) error
	// WaitCleanupBeforeKubeAPIServer waits until the extensions that should be handled before the kube-apiserver are cleaned up.
	WaitCleanupBeforeKubeAPIServer(context.Context) error

	// DestroyAfterKubeAPIServer deletes the extensions that should be handled after the kube-apiserver.
	DestroyAfterKubeAPIServer(context.Context) error
	// WaitCleanupAfterKubeAPIServer waits until the extensions that should be handled after the kube-apiserver are cleaned up.
	WaitCleanupAfterKubeAPIServer(context.Context) error

	// WaitCleanup waits until all extensions are cleaned up.
	WaitCleanup(ctx context.Context) error

	// MigrateBeforeKubeAPIServer migrates all Extension resources that should be handled before the kube-apiserver.
	MigrateBeforeKubeAPIServer(ctx context.Context) error
	// WaitMigrateBeforeKubeAPIServer waits until all Extension resources that should be handled before the kube-apiserver are migrated.
	WaitMigrateBeforeKubeAPIServer(ctx context.Context) error

	// MigrateAfterKubeAPIServer migrates all Extension resources that should be handled after the kube-apiserver.
	MigrateAfterKubeAPIServer(ctx context.Context) error
	// WaitMigrateAfterKubeAPIServer waits until all Extension resources that should be handled after the kube-apiserver are migrated.
	WaitMigrateAfterKubeAPIServer(ctx context.Context) error
}

// Extension contains information about the desired Extension resources as well as configuration information.
type Extension struct {
	extensionsv1alpha1.Extension
	// Timeout is the maximum waiting time for the Extension status to report readiness.
	Timeout time.Duration
	// Lifecycle defines when an extension resource should be updated during different operations.
	Lifecycle *gardencorev1beta1.ControllerResourceLifecycle
}

// Values contains the values used to create an Extension resources.
type Values struct {
	// Namespace is the namespace into which the Extension resources should be deployed.
	Namespace string
	// Extensions is the map of extensions where the key is the type and the value is an Extension structure.
	Extensions map[string]Extension
}

type extension struct {
	values              *Values
	log                 logr.Logger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	extensions map[string]*extensionsv1alpha1.Extension
}

// New creates a new instance of Extension deployer.
func New(
	log logr.Logger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {
	return &extension{
		values:              values,
		log:                 log,
		client:              client,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,

		extensions: make(map[string]*extensionsv1alpha1.Extension),
	}
}

// DeployAfterKubeAPIServer uses the seed client to create or update the Extension resources that should be deployed after the kube-apiserver.
func (e *extension) DeployAfterKubeAPIServer(ctx context.Context) error {
	fns := e.forEach(func(ctx context.Context, ext *extensionsv1alpha1.Extension, extType string, providerConfig *runtime.RawExtension, _ time.Duration) error {
		_, err := e.deploy(ctx, ext, extType, providerConfig, v1beta1constants.GardenerOperationReconcile)
		return err
	}, deployAfterKubeAPIServer)

	return flow.Parallel(fns...)(ctx)
}

// DeployBeforeKubeAPIServer uses the seed client to create or update the Extension resources that should be deployed before the kube-apiserver.
func (e *extension) DeployBeforeKubeAPIServer(ctx context.Context) error {
	fns := e.forEach(func(ctx context.Context, ext *extensionsv1alpha1.Extension, extType string, providerConfig *runtime.RawExtension, _ time.Duration) error {
		_, err := e.deploy(ctx, ext, extType, providerConfig, v1beta1constants.GardenerOperationReconcile)
		return err
	}, deployBeforeKubeAPIServer)

	return flow.Parallel(fns...)(ctx)
}

func (e *extension) deploy(ctx context.Context, ext *extensionsv1alpha1.Extension, extType string, providerConfig *runtime.RawExtension, operation string) (extensionsv1alpha1.Object, error) {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, e.client, ext, func() error {
		metav1.SetMetaDataAnnotation(&ext.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&ext.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())
		ext.Spec.Type = extType
		ext.Spec.ProviderConfig = providerConfig
		return nil
	})
	return ext, err
}

// DestroyBeforeKubeAPIServer deletes all Extension resources that should be handled before the kube-apiserver.
func (e *extension) DestroyBeforeKubeAPIServer(ctx context.Context) error {
	extensionsBeforeKAPI := e.filterExtensions(deleteBeforeKubeAPIServer)
	return e.deleteExtensionResources(ctx, func(obj extensionsv1alpha1.Object) bool {
		return extensionsBeforeKAPI.Has(obj.GetExtensionSpec().GetExtensionType())
	})
}

// DestroyAfterKubeAPIServer deletes all Extension resources that should be handled after the kube-apiserver.
func (e *extension) DestroyAfterKubeAPIServer(ctx context.Context) error {
	extensionsAfterKAPI := e.filterExtensions(deleteAfterKubeAPIServer)
	return e.deleteExtensionResources(ctx, func(obj extensionsv1alpha1.Object) bool {
		return extensionsAfterKAPI.Has(obj.GetExtensionSpec().GetExtensionType())
	})
}

// WaitAfterKubeAPIServer waits until the Extension resources that should be deployed after the kube-apiserver are ready.
func (e *extension) WaitAfterKubeAPIServer(ctx context.Context) error {
	fns := e.forEach(func(ctx context.Context, ext *extensionsv1alpha1.Extension, _ string, _ *runtime.RawExtension, timeout time.Duration) error {
		return extensions.WaitUntilExtensionObjectReady(
			ctx,
			e.client,
			e.log,
			ext,
			extensionsv1alpha1.ExtensionResource,
			e.waitInterval,
			e.waitSevereThreshold,
			timeout,
			nil,
		)
	}, deployAfterKubeAPIServer)

	return flow.ParallelExitOnError(fns...)(ctx)
}

// Wait waits until the Extension resources that should be deployed before the kube-apiserver are ready.
func (e *extension) WaitBeforeKubeAPIServer(ctx context.Context) error {
	fns := e.forEach(func(ctx context.Context, ext *extensionsv1alpha1.Extension, _ string, _ *runtime.RawExtension, timeout time.Duration) error {
		return extensions.WaitUntilExtensionObjectReady(
			ctx,
			e.client,
			e.log,
			ext,
			extensionsv1alpha1.ExtensionResource,
			e.waitInterval,
			e.waitSevereThreshold,
			timeout,
			nil,
		)
	}, deployBeforeKubeAPIServer)

	return flow.ParallelExitOnError(fns...)(ctx)
}

// WaitCleanup waits until the Extension resources are cleaned up.
func (e *extension) WaitCleanup(ctx context.Context) error {
	return e.waitCleanup(ctx, nil)
}

// WaitCleanupBeforeKubeAPIServer waits until all Extension resources that are handled before the kube-apiserver are cleaned up.
func (e *extension) WaitCleanupBeforeKubeAPIServer(ctx context.Context) error {
	extensionsBeforeKAPI := e.filterExtensions(deleteBeforeKubeAPIServer)
	return e.waitCleanup(ctx, func(obj extensionsv1alpha1.Object) bool {
		return extensionsBeforeKAPI.Has(obj.GetExtensionSpec().GetExtensionType())
	})
}

// WaitCleanupAfterKubeAPIServer waits until all Extension resources that are handled after the kube-apiserver are cleaned up.
func (e *extension) WaitCleanupAfterKubeAPIServer(ctx context.Context) error {
	extensionsAfterKAPI := e.filterExtensions(deleteAfterKubeAPIServer)
	return e.waitCleanup(ctx, func(obj extensionsv1alpha1.Object) bool {
		return extensionsAfterKAPI.Has(obj.GetExtensionSpec().GetExtensionType())
	})
}

// RestoreAfterKubeAPIServer uses the seed client and the ShootState to create the Extension resources that should be deployed after the kube-apiserver and restore their state.
func (e *extension) RestoreAfterKubeAPIServer(ctx context.Context, shootState *gardencorev1beta1.ShootState) error {
	fns := e.forEach(func(ctx context.Context, ext *extensionsv1alpha1.Extension, extType string, providerConfig *runtime.RawExtension, _ time.Duration) error {
		return extensions.RestoreExtensionWithDeployFunction(
			ctx,
			e.client,
			shootState,
			extensionsv1alpha1.ExtensionResource,
			func(ctx context.Context, operationAnnotation string) (extensionsv1alpha1.Object, error) {
				return e.deploy(ctx, ext, extType, providerConfig, operationAnnotation)
			},
		)
	}, deployAfterKubeAPIServer)

	return flow.Parallel(fns...)(ctx)
}

// Restore uses the seed client and the ShootState to create the Extension resources that should be deployed before the kube-apiserver and restore their state.
func (e *extension) RestoreBeforeKubeAPIServer(ctx context.Context, shootState *gardencorev1beta1.ShootState) error {
	fns := e.forEach(func(ctx context.Context, ext *extensionsv1alpha1.Extension, extType string, providerConfig *runtime.RawExtension, _ time.Duration) error {
		return extensions.RestoreExtensionWithDeployFunction(
			ctx,
			e.client,
			shootState,
			extensionsv1alpha1.ExtensionResource,
			func(ctx context.Context, operationAnnotation string) (extensionsv1alpha1.Object, error) {
				return e.deploy(ctx, ext, extType, providerConfig, operationAnnotation)
			},
		)
	}, deployBeforeKubeAPIServer)

	return flow.Parallel(fns...)(ctx)
}

// MigrateBeforeKubeAPIServer migrates all Extension resources that should be handled before the kube-apiserver.
func (e *extension) MigrateBeforeKubeAPIServer(ctx context.Context) error {
	extensionsBeforeKAPI := e.filterExtensions(migrateBeforeKubeAPIServer)
	return extensions.MigrateExtensionObjects(
		ctx,
		e.client,
		&extensionsv1alpha1.ExtensionList{},
		e.values.Namespace,
		func(obj extensionsv1alpha1.Object) bool {
			return extensionsBeforeKAPI.Has(obj.GetExtensionSpec().GetExtensionType())
		},
	)
}

// WaitMigrateBeforeKubeAPIServer waits until all Extension resources that should be handled before the kube-apiserver are migrated.
func (e *extension) WaitMigrateBeforeKubeAPIServer(ctx context.Context) error {
	extensionsBeforeKAPI := e.filterExtensions(migrateBeforeKubeAPIServer)
	return extensions.WaitUntilExtensionObjectsMigrated(
		ctx,
		e.client,
		&extensionsv1alpha1.ExtensionList{},
		extensionsv1alpha1.ExtensionResource,
		e.values.Namespace,
		e.waitInterval,
		e.waitTimeout,
		func(obj extensionsv1alpha1.Object) bool {
			return extensionsBeforeKAPI.Has(obj.GetExtensionSpec().GetExtensionType())
		},
	)
}

// MigrateAfterKubeAPIServer migrates all Extension resources that should be handled after the kube-apiserver.
func (e *extension) MigrateAfterKubeAPIServer(ctx context.Context) error {
	extensionsAfterKAPI := e.filterExtensions(migrateAfterKubeAPIServer)
	return extensions.MigrateExtensionObjects(
		ctx,
		e.client,
		&extensionsv1alpha1.ExtensionList{},
		e.values.Namespace,
		func(obj extensionsv1alpha1.Object) bool {
			return extensionsAfterKAPI.Has(obj.GetExtensionSpec().GetExtensionType())
		},
	)
}

// WaitMigrateAfterKubeAPIServer waits until all Extension resources that should be handled after the kube-apiserver are migrated.
func (e *extension) WaitMigrateAfterKubeAPIServer(ctx context.Context) error {
	extensionsAfterKAPI := e.filterExtensions(migrateAfterKubeAPIServer)
	return extensions.WaitUntilExtensionObjectsMigrated(
		ctx,
		e.client,
		&extensionsv1alpha1.ExtensionList{},
		extensionsv1alpha1.ExtensionResource,
		e.values.Namespace,
		e.waitInterval,
		e.waitTimeout,
		func(obj extensionsv1alpha1.Object) bool {
			return extensionsAfterKAPI.Has(obj.GetExtensionSpec().GetExtensionType())
		},
	)
}

// DeleteStaleResources deletes unused Extension resources from the shoot namespace in the seed.
func (e *extension) DeleteStaleResources(ctx context.Context) error {
	wantedExtensionTypes := e.getWantedExtensionTypes()
	return e.deleteExtensionResources(ctx, func(obj extensionsv1alpha1.Object) bool {
		return !wantedExtensionTypes.Has(obj.GetExtensionSpec().GetExtensionType())
	})
}

// WaitCleanupStaleResources waits until all unused Extension resources are cleaned up.
func (e *extension) WaitCleanupStaleResources(ctx context.Context) error {
	wantedExtensionTypes := e.getWantedExtensionTypes()
	return e.waitCleanup(ctx, func(obj extensionsv1alpha1.Object) bool {
		return !wantedExtensionTypes.Has(obj.GetExtensionSpec().GetExtensionType())
	})
}

func (e *extension) deleteExtensionResources(ctx context.Context, predicate func(obj extensionsv1alpha1.Object) bool) error {
	return extensions.DeleteExtensionObjects(
		ctx,
		e.client,
		&extensionsv1alpha1.ExtensionList{},
		e.values.Namespace,
		predicate,
	)
}

func (e *extension) waitCleanup(ctx context.Context, predicate func(obj extensionsv1alpha1.Object) bool) error {
	return extensions.WaitUntilExtensionObjectsDeleted(
		ctx,
		e.client,
		e.log,
		&extensionsv1alpha1.ExtensionList{},
		extensionsv1alpha1.ExtensionResource,
		e.values.Namespace,
		e.waitInterval,
		e.waitTimeout,
		predicate,
	)
}

// getWantedExtensionTypes returns the types of all extension resources, that are currently needed based
// on the configured shoot settings and globally enabled extensions.
func (e *extension) getWantedExtensionTypes() sets.Set[string] {
	wantedExtensionTypes := sets.New[string]()
	for _, ext := range e.values.Extensions {
		wantedExtensionTypes.Insert(ext.Spec.Type)
	}
	return wantedExtensionTypes
}

func (e *extension) forEach(
	fn func(ctx context.Context, ext *extensionsv1alpha1.Extension, extType string, providerConfig *runtime.RawExtension, timeout time.Duration) error,
	f filter,
) []flow.TaskFn {
	fns := make([]flow.TaskFn, 0, len(e.values.Extensions))

	for _, ext := range e.values.Extensions {
		if !f(ext) {
			continue
		}
		extensionTemplate := ext

		extensionObj, ok := e.extensions[extensionTemplate.Name]
		if !ok {
			extensionObj = &extensionsv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name:      extensionTemplate.Name,
					Namespace: e.values.Namespace,
				},
			}
			// store object for later usage (we want to pass a filled object to WaitUntil*)
			e.extensions[extensionTemplate.Name] = extensionObj
		}

		fns = append(fns, func(ctx context.Context) error {
			return fn(ctx, extensionObj, extensionTemplate.Spec.Type, extensionTemplate.Spec.ProviderConfig, extensionTemplate.Timeout)
		})
	}

	return fns
}

// Extensions returns the map of extensions where the key is the type and the value is an Extension structure.
func (e *extension) Extensions() map[string]Extension {
	return e.values.Extensions
}
