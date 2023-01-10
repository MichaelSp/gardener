# Checklist For Adding New Components

Adding new components which run in garden, seed or shoot cluster is theoretically quite simple - we just need a `Deployment` (or similar other workload resource), the respective container image and maybe a bit of configuration.
In practice however, there are a couple of things to keep in mind in order to make the deployment production-ready.
This document provides a checklist for them which you can walk through. 

## General

1. **Avoid usage of Helm charts** ([example](https://github.com/gardener/gardener/tree/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/metricsserver))

   Nowadays, we use [Golang components](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/interfaces.go) instead of Helm charts for deploying components to a cluster.
   Please find a typical structure of such components [here](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/metricsserver/metrics_server.go#L80-L97) (configuration values are typically managed in a `Values` structure).
   There are a few exceptions (e.g., [Istio](https://github.com/gardener/gardener/tree/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/istio)) still using charts, however the default should be using a Golang-based implementation.
   For the exceptional cases, use Golang's [embed](https://pkg.go.dev/embed) package to embed the Helm chart directory ([example 1](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/istio/istiod.go#L51-L52), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/istio/istiod.go#L257-L273)). 

2. **Choose the proper deployment way** ([example 1 (direct application w/ client)](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/kubescheduler/kube_scheduler.go#L210-L225), [example 2 (using `ManagedResource`)](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/kubescheduler/kube_scheduler.go#L442-L484), [example 3 (mixed scenario)](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/kubestatemetrics/kube_state_metrics.go#L116))

   For historic reasons, resources related to shoot control plane components are applied directly with the client.
   All other resources (seed or shoot system components) are deployed via `gardener-resource-manager`'s [Resource controller](../concepts/resource-manager.md#managedresource-controller) (`ManagedResource`s) since it performs health checks out-of-the-box and has a lot of other features (see its documentation for more information).
   Components which can run as both seed system component or shoot control plane component (e.g., VPA or `kube-state-metrics`) can make use of [these utility functions](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/resourceconfig.go).

3. **Do not hard-code container image references** ([example 1](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/charts/images.yaml#L130-L133), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/metricsserver.go#L28-L31), [example 3](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/metricsserver/metrics_server.go#L82-L83))

   We define all image references centrally in the [`charts/images.yaml`](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/charts/images.yaml) file.
   Hence, the image references must not be hard-coded in the pod template spec but read from this so-called [image vector](../deployment/image_vector.md) instead.

4. **Use unique `ConfigMap`s/`Secret`s** ([example 1](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/kubescheduler/kube_scheduler.go#L181-L188), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/kubescheduler/kube_scheduler.go#L347))

   [Unique `ConfigMap`s/`Secret`s](https://kubernetes.io/docs/concepts/configuration/configmap/#configmap-immutable) are immutable for modification and have a unique name.
   This has a couple of benefits, e.g. the `kubelet` doesn't watch these resources, and it is always clear which resource contains which data since it cannot be changed.
   As a consequence, unique/immutable `ConfigMap`s/`Secret` are superior to checksum annotations on the pod templates.
   Stale/unused `ConfigMap`s/`Secret`s are garbage-collected by `gardener-resource-manager`'s [GarbageCollector](../concepts/resource-manager.md#garbage-collector-for-immutable-configmapssecrets).
   There are utility functions (see examples above) for using unique `ConfigMap`s/`Secret`s in Golang components.
   It is essential to inject the annotations into the workload resource to make the garbage-collection work.\
   Note that some `ConfigMap`s/`Secret`s should not be unique (e.g., those containing monitoring or logging configuration).
   The reason is that the old revision stays in the cluster even if unused until the garbage-collector acts.
   During this time, they would be wrongly aggregated to the full configuration.

5. **Manage certificates/secrets via [secrets manager](https://github.com/gardener/gardener/tree/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/utils/secrets/manager)** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/metricsserver/metrics_server.go#L100-L109))

   You should use the [secrets manager](secrets_management.md) for the management of any kind of credentials.
   This makes sure that credentials rotation works out-of-the-box without you requiring to think about it.
   Generally, do not use client certificates (see [security section](#security)).

6. **Consider hibernation when calculating replica count** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/kubescheduler.go#L36))

   Shoot clusters can be [hibernated](../usage/shoot_hibernate.md) meaning that all control plane components in the shoot namespace in the seed cluster are scaled down to zero and all worker nodes are terminated.
   If your component runs in the seed cluster then you have to consider this case and provide the proper replica count.
   There is a utility function available (see example).

7. **Ensure task dependencies are as precise as possible in shoot flows** ([example 1](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/gardenlet/controller/shoot/shoot/reconciler_reconcile.go#L508-L512), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/gardenlet/controller/shoot/shoot/reconciler_delete.go#L368-L372))

   Only define the minimum of needed dependency tasks in the [shoot reconciliation/deletion flows](https://github.com/gardener/gardener/tree/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/gardenlet/controller/shoot/shoot).

8. **Handle shoot system components**

   Shoot system components deployed by `gardener-resource-manager` are labelled with `resource.gardener.cloud/managed-by: gardener`. This makes Gardener adding required label selectors and tolerations so that non-`DaemonSet` managed `Pod`s will exclusively run on selected nodes, [more information](../concepts/resource-manager.md#system-components-webhook).
   `DaemonSet`s on the other hand, should generally tolerate any `NoSchedule` or `NoExecute` taints so that they can run on any `Node`, regardless of user added taints.

## Security

1. **Use a [dedicated `ServiceAccount`](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/) and disable auto-mount** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/metricsserver/metrics_server.go#L145-L151))

   Components which need to talk to the API server of their runtime cluster must always use a dedicated `ServiceAccount` (do not use `default`) which `automountServiceAccountToken` set to `false`.
   This makes `gardener-resource-manager`'s [TokenInvalidator](../concepts/resource-manager.md#tokeninvalidator) invalidating the static token secret and its [`ProjectedTokenMount` webhook](../concepts/resource-manager.md#auto-mounting-projected-serviceaccount-tokens) injecting a projected token automatically.

2. **Use shoot access tokens instead of a client certificates** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/kubescheduler/kube_scheduler.go#L227-L229))

   Components which need to talk to a target cluster different from their runtime cluster (e.g., running in seed cluster but talking to shoot) then the `gardener-resource-manager`'s [TokenRequestor](../concepts/resource-manager.md#tokenrequestor) should be used to manage a so-called "shoot access token".

3. **Define RBAC roles with minimal privileges** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/metricsserver/metrics_server.go#L153-L223))

   The component's `ServiceAccount` (if exists) should have as little privileges as possible.
   Consequently, please define proper [RBAC roles](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) for it.
   This might include a combination of `ClusterRole`s and `Role`s.
   Please do not provide elevated privileges due to laziness (e.g., because there is already a `ClusterRole` that can be extended vs. creating a `Role` only when only access to a single namespace is needed).

4. **Use [`NetworkPolicy`s](https://kubernetes.io/docs/concepts/services-networking/network-policies/) to restrict network traffic** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/etcd/etcd.go#L293-L339))

   You should restrict both ingress and egress traffic to/from your component as much as possible to ensure that it only gets access to/from other components if really needed.
   Gardener provides a few default policies for typical usage scenarios, please see [this document for seed clusters](seed_network_policies.md) and [this document for shoot clusters](../usage/shoot_network_policies.md).

5. **Do not run components in privileged mode** ([example 1](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/nodelocaldns/nodelocaldns.go#L329-L333), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/nodelocaldns/nodelocaldns.go#L507))

   Avoid running components with `privileged=true` and define the needed [Linux capabilities](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-capabilities-for-a-container) instead.

6. **Choose the proper Seccomp profile** ([example 1](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/nodelocaldns/nodelocaldns.go#L285-L287), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/nginxingress/nginxingress.go#L427))

   The [Seccomp profile](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-seccomp-profile-for-a-container) will be defaulted by `gardener-resource-manager`'s SeccompProfile webhook which works well for the majority of components.
   However, in some special cases you might need to overwrite it.

7. **Define `PodSecurityPolicy`s** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/vpnshoot/vpnshoot.go#L445-L516))

   `PodSecurityPolicy`s are deprecated, however Gardener still supports shoot clusters with older Kubernetes versions ([ref](../usage/supported_k8s_versions.md)).
   To make sure that such clusters can run with `.spec.kubernetes.allowPrivilegedContainers=false`, you have to define proper `PodSecurityPolicy`s.
   See also [this document](../usage/pod-security.md) for more information.

## High Availability / Stability

1. **Specify the component type label for high availability** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/kubescheduler/kube_scheduler.go#L234))

   To support high-availability deployments, `gardener-resource-manager`s [HighAvailabilityConfig](../concepts/resource-manager.md#high-availability-config) webhook injects the proper specification like replica or topology spread constraints.
   You only need to specify the type label, see also [this document](high-availability.md) for more information.

2. **Define a `PodDisruptionBudget`** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/metricsserver/metrics_server.go#L398-L422))

   Closely related to high availability but also to stability in general: The definition of a [`PodDisruptionBudget`](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) with `maxUnavailable=1` should be provided by default.

3. **Choose the right `PriorityClass`** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/kubescheduler/kube_scheduler.go#L301))

   Each cluster runs many components with different priorities.
   Gardener provides a set of default [`PriorityClass`es](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass), see [this document](priority-classes.md) for more information.

4. **Consider defining liveness and readiness probes** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/metricsserver/metrics_server.go#L335-L358))

   To ensure smooth rolling update behaviour, consider the definition of [liveness and/or readiness probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/).

## Scalability

1. **Provide resource requirements** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/metricsserver/metrics_server.go#L359-L367))

   All components should have [resource requirements](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits).
   Generally, they should always request CPU and memory while only memory shall be limited (no CPU limits!).

2. **Define a `VerticalPodAutoscaler`** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/metricsserver/metrics_server.go#L424-L460))

   We typically perform vertical auto-scaling via the VPA managed by the [Kubernetes community](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler).
   Each component should have a respective `VerticalPodAutoscaler` which "min allowed" resources, "auto update mode", and "requests only"-mode.
   VPA is always enabled in garden or seed clusters while it is optional for shoot clusters.

3. **Define a `HorizontalPodAutoscaler` if needed** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/coredns/coredns.go#L689-L738))

   If your component is capable of scaling horizontally, the definition of a [`HorizontalPodAutoscaler`](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) should be considered.

## Observability / Operations Productivity

1. **Provide monitoring scrape config and alerting rules** ([example 1](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/coredns/monitoring.go), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/monitoring.go#L97))

   Components should provide scrape configuration and alerting rules for Prometheus/Alertmanager if appropriate.
   This should be done inside a dedicated `monitoring.go` file.
   Extensions should follow [this document](../extensions/logging-and-monitoring.md#extensions-monitoring-integration).

2. **Provide logging parsers and filters** ([example 1](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/coredns/logging.go), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/gardenlet/controller/seed/seed/reconciler_reconcile.go#L563))

   Components should provide parsers and filters for fluent-bit if appropriate.
   This should be done inside a dedicated `logging.go` file.
   Extensions should follow [this document](../extensions/logging-and-monitoring.md#fluent-bit-log-parsers-and-filters).

3. **Set the `revisionHistoryLimit` to `2` for `Deployment`s** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/metricsserver/metrics_server.go#L273))

   In order to allow easy inspection of two `ReplicaSet`s to quickly find the changes that lead to a rolling update, the [revision history limit](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#revision-history-limit) should be set to `2`.

4. **Define health checks** ([example 1](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/care/checker.go#L45-L71), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/care/seed_health.go#L46-L54))

   `gardenlet`'s [care controllers](../concepts/gardenlet.md#controllers) regularly check the health status of system or control plane components.
   You need to enhance the lists of components to check if your component related to the seed system or shoot control plane (shoot system components are automatically checked via their respective [`ManagedResource` conditions](../concepts/resource-manager.md#managedresource-controller)), see examples above.

5. **Configure automatic restarts in shoot maintenance time window** ([example 1](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/component/kubescheduler/kube_scheduler.go#L243), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/coredns.go#L90-L107))

   Gardener offers to restart components during the maintenance time window, see [this document](../usage/shoot_maintenance.md#restart-control-plane-controllers) and [this document](../usage/shoot_maintenance.md#restart-some-core-addons).
   You can consider adding the needed label to your control plane component to get this automatic restart (probably not needed for most components).