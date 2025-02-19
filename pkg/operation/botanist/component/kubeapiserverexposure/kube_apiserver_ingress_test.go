// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserverexposure_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserverexposure"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#Ingress", func() {
	var (
		ctx context.Context
		c   client.Client

		ingressObjKey    client.ObjectKey
		ingressNamespace string
		ingressClass     string
		pathType         networkingv1.PathType
		expected         *networkingv1.Ingress
		defaultDeployer  component.Deployer
	)

	BeforeEach(func() {
		ctx = context.TODO()
		s := runtime.NewScheme()
		Expect(networkingv1.AddToScheme(s)).To(Succeed())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		ingressNamespace = "bar"
		ingressObjKey = client.ObjectKey{Name: "kube-apiserver", Namespace: ingressNamespace}
		pathType = networkingv1.PathTypePrefix
		ingressClass = "foo-bar-ingress"
		expected = &networkingv1.Ingress{
			TypeMeta: metav1.TypeMeta{
				APIVersion: networkingv1.SchemeGroupVersion.String(),
				Kind:       "Ingress",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      ingressObjKey.Name,
				Namespace: ingressObjKey.Namespace,
				Annotations: map[string]string{
					"nginx.ingress.kubernetes.io/ssl-passthrough": "true",
				},
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: &ingressClass,
				Rules: []networkingv1.IngressRule{
					{
						Host: "foo.bar.example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "foo",
												Port: networkingv1.ServiceBackendPort{
													Number: 443,
												},
											},
										},
										Path:     "/",
										PathType: &pathType,
									},
								},
							},
						},
					},
				},
				TLS: []networkingv1.IngressTLS{{Hosts: []string{"foo.bar.example.com"}}},
			},
		}
	})

	JustBeforeEach(func() {
		defaultDeployer = NewIngress(c, ingressNamespace, IngressValues{
			Host:             "foo.bar.example.com",
			IngressClassName: &ingressClass,
			ServiceName:      "foo",
		})
	})

	Context("Deploy", func() {
		It("should create the expected ingress object", func() {
			Expect(defaultDeployer.Deploy(ctx)).To(Succeed())

			actual := &networkingv1.Ingress{}
			Expect(c.Get(ctx, ingressObjKey, actual)).To(Succeed())
			Expect(actual.Annotations).To(DeepEqual(expected.Annotations))
			Expect(actual.Labels).To(DeepEqual(expected.Labels))
			Expect(actual.Spec).To(DeepEqual(expected.Spec))
		})
	})

	Context("Destroy", func() {
		It("should delete the ingress object", func() {
			Expect(c.Create(ctx, expected)).To(Succeed())
			Expect(c.Get(ctx, ingressObjKey, &networkingv1.Ingress{})).To(Succeed())

			Expect(defaultDeployer.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, ingressObjKey, &networkingv1.Ingress{})).To(BeNotFoundError())
		})
	})
})
