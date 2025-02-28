/*
Copyright 2021 The KubeVela Authors.

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

package definition

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/google/go-cmp/cmp"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"

	"github.com/oam-dev/kubevela/pkg/dsl/model"
	"github.com/oam-dev/kubevela/pkg/oam/discoverymapper"
	"github.com/oam-dev/kubevela/pkg/oam/util"
)

var _ = Describe("Package discovery resources for definition from K8s APIServer", func() {

	It("discovery built-in k8s resource", func() {

		By("test ingress in kube package")
		bi := build.NewContext().NewInstance("", nil)
		pd.ImportBuiltinPackagesFor(bi)
		err := bi.AddFile("-", `
import (
	network "k8s.io/networking/v1beta1"
	kube	"kube/networking.k8s.io/v1beta1"
)
output: network.#Ingress & kube.#Ingress
output: {
	apiVersion: "networking.k8s.io/v1beta1"
	kind:       "Ingress"
	metadata: name: "myapp"
	spec: {
		rules: [{
			host: parameter.domain
			http: {
				paths: [
					for k, v in parameter.http {
						path: k
						backend: {
							serviceName: "myname"
							servicePort: v
						}
					},
				]
			}
		}]
	}
}
parameter: {
	domain: "abc.com"
	http: {
		"/": 80
	}
}`)
		Expect(err).ToNot(HaveOccurred())
		var r cue.Runtime
		inst, err := r.Build(bi)
		Expect(err).Should(BeNil())
		base, err := model.NewBase(inst.Lookup("output"))
		Expect(err).Should(BeNil())
		data, err := base.Unstructured()
		Expect(err).Should(BeNil())

		Expect(cmp.Diff(data, &unstructured.Unstructured{Object: map[string]interface{}{
			"kind":       "Ingress",
			"apiVersion": "networking.k8s.io/v1beta1",
			"metadata":   map[string]interface{}{"name": "myapp"},
			"spec": map[string]interface{}{
				"rules": []interface{}{
					map[string]interface{}{
						"host": "abc.com",
						"http": map[string]interface{}{
							"paths": []interface{}{
								map[string]interface{}{
									"path": "/",
									"backend": map[string]interface{}{
										"serviceName": "myname",
										"servicePort": int64(80),
									}}}}}}}},
		})).Should(BeEquivalentTo(""))
		By("test Invalid Import path")
		bi = build.NewContext().NewInstance("", nil)
		pd.ImportBuiltinPackagesFor(bi)
		bi.AddFile("-", `
import (
	"k8s.io/networking/v1"
	kube	"kube/networking.k8s.io/v1"
)
output: v1.#Deployment & kube.#Deployment
output: {
	metadata: {
		"name": parameter.name
	}
	spec: template: spec: {
		containers: [{
			name:"invalid-path",
			image: parameter.image
		}]
	}
}

parameter: {
	name:  "myapp"
	image: "nginx"
}`)
		inst, err = r.Build(bi)
		Expect(err).Should(BeNil())
		_, err = model.NewBase(inst.Lookup("output"))
		Expect(err).ShouldNot(BeNil())
		Expect(err.Error()).Should(Equal("_|_ // undefined field \"#Deployment\""))

		By("test Deployment in kube package")
		bi = build.NewContext().NewInstance("", nil)
		pd.ImportBuiltinPackagesFor(bi)
		bi.AddFile("-", `
import (
	apps "k8s.io/apps/v1"
	kube	"kube/apps/v1"
)
output: apps.#Deployment & kube.#Deployment
output: {
	metadata: {
		"name": parameter.name
	}
	spec: template: spec: {
		containers: [{
			name:"test",
			image: parameter.image
		}]
	}
}
parameter: {
	name:  "myapp"
	image: "nginx"
}`)
		inst, err = r.Build(bi)
		Expect(err).Should(BeNil())
		base, err = model.NewBase(inst.Lookup("output"))
		Expect(err).Should(BeNil())
		data, err = base.Unstructured()
		Expect(err).Should(BeNil())
		Expect(cmp.Diff(data, &unstructured.Unstructured{Object: map[string]interface{}{
			"kind":       "Deployment",
			"apiVersion": "apps/v1",
			"metadata":   map[string]interface{}{"name": "myapp"},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{},
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test",
								"image": "nginx"}}}}}},
		})).Should(BeEquivalentTo(""))

		By("test Secret in kube package")
		bi = build.NewContext().NewInstance("", nil)
		pd.ImportBuiltinPackagesFor(bi)
		bi.AddFile("-", `
import (
	"k8s.io/core/v1"
	kube "kube/v1"
)
output: v1.#Secret & kube.#Secret
output: {
	metadata: {
		"name": parameter.name
	}
	type:"kubevela"
}
parameter: {
	name:  "myapp"
}`)
		inst, err = r.Build(bi)
		Expect(err).Should(BeNil())
		base, err = model.NewBase(inst.Lookup("output"))
		Expect(err).Should(BeNil())
		data, err = base.Unstructured()
		Expect(err).Should(BeNil())
		Expect(cmp.Diff(data, &unstructured.Unstructured{Object: map[string]interface{}{
			"kind":       "Secret",
			"apiVersion": "v1",
			"metadata":   map[string]interface{}{"name": "myapp"},
			"type":       "kubevela"}})).Should(BeEquivalentTo(""))

		By("test Service in kube package")
		bi = build.NewContext().NewInstance("", nil)
		pd.ImportBuiltinPackagesFor(bi)
		bi.AddFile("-", `
import (
	"k8s.io/core/v1"
	kube "kube/v1"
)
output: v1.#Service & kube.#Service
output: {
	metadata: {
		"name": parameter.name
	}
	spec: type: "ClusterIP",
}
parameter: {
	name:  "myapp"
}`)
		inst, err = r.Build(bi)
		Expect(err).Should(BeNil())
		base, err = model.NewBase(inst.Lookup("output"))
		Expect(err).Should(BeNil())
		data, err = base.Unstructured()
		Expect(err).Should(BeNil())
		Expect(cmp.Diff(data, &unstructured.Unstructured{Object: map[string]interface{}{
			"kind":       "Service",
			"apiVersion": "v1",
			"metadata":   map[string]interface{}{"name": "myapp"},
			"spec": map[string]interface{}{
				"type": "ClusterIP"}},
		})).Should(BeEquivalentTo(""))
		Expect(pd.Exist(metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Service",
		})).Should(Equal(true))

		By("Check newly added CRD refreshed and could be used in CUE package")
		crd1 := crdv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo.example.com",
			},
			Spec: crdv1.CustomResourceDefinitionSpec{
				Group: "example.com",
				Names: crdv1.CustomResourceDefinitionNames{
					Kind:     "Foo",
					ListKind: "FooList",
					Plural:   "foo",
					Singular: "foo",
				},
				Versions: []crdv1.CustomResourceDefinitionVersion{{
					Name:         "v1",
					Served:       true,
					Storage:      true,
					Subresources: &crdv1.CustomResourceSubresources{Status: &crdv1.CustomResourceSubresourceStatus{}},
					Schema: &crdv1.CustomResourceValidation{
						OpenAPIV3Schema: &crdv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]crdv1.JSONSchemaProps{
								"spec": {
									Type:                   "object",
									XPreserveUnknownFields: pointer.BoolPtr(true),
									Properties: map[string]crdv1.JSONSchemaProps{
										"key": {Type: "string"},
									}},
								"status": {
									Type:                   "object",
									XPreserveUnknownFields: pointer.BoolPtr(true),
									Properties: map[string]crdv1.JSONSchemaProps{
										"key":      {Type: "string"},
										"app-hash": {Type: "string"},
									}}}}}},
				},
				Scope: crdv1.NamespaceScoped,
			},
		}
		Expect(k8sClient.Create(context.Background(), &crd1)).Should(SatisfyAny(BeNil(), &util.AlreadyExistMatcher{}))

		mapper, err := discoverymapper.New(cfg)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(func() error {
			_, err := mapper.RESTMapping(schema.GroupKind{Group: "example.com", Kind: "Foo"}, "v1")
			return err
		}, time.Second*2, time.Millisecond*300).Should(BeNil())

		Expect(pd.Exist(metav1.GroupVersionKind{
			Group:   "example.com",
			Version: "v1",
			Kind:    "Foo",
		})).Should(Equal(false))

		By("test new added CRD in kube package")
		Eventually(func() error {
			if err := pd.RefreshKubePackagesFromCluster(); err != nil {
				return err
			}
			bi = build.NewContext().NewInstance("", nil)
			pd.ImportBuiltinPackagesFor(bi)
			if err := bi.AddFile("-", `
import (
	ev1 "example.com/v1"
	kv1 "kube/example.com/v1"
)
output: ev1.#Foo & kv1.#Foo
output: {
	spec: key: "test1"
    status: key: "test2"
}
`); err != nil {
				return err
			}
			inst, err = r.Build(bi)
			if err != nil {
				return err
			}
			return nil
		}, time.Second*5, time.Millisecond*300).Should(BeNil())

		base, err = model.NewBase(inst.Lookup("output"))
		Expect(err).Should(BeNil())
		data, err = base.Unstructured()
		Expect(err).Should(BeNil())
		Expect(cmp.Diff(data, &unstructured.Unstructured{Object: map[string]interface{}{
			"kind":       "Foo",
			"apiVersion": "example.com/v1",
			"spec": map[string]interface{}{
				"key": "test1"},
			"status": map[string]interface{}{
				"key": "test2"}},
		})).Should(BeEquivalentTo(""))

	})

})
