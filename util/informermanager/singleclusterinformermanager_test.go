package informermanager

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
)

func TestSingleClusterInformerManager(t *testing.T) {
	scenarios := []struct {
		name        string
		existingObj *unstructured.Unstructured
		gvr         schema.GroupVersionResource
		ns          string
		trigger     func(gvr schema.GroupVersionResource, ns string, fakeClient *fake.FakeDynamicClient, testObject *unstructured.Unstructured) *unstructured.Unstructured
		handler     func(rcvCh chan<- *unstructured.Unstructured) *cache.ResourceEventHandlerFuncs
	}{
		// scenario 1
		{
			name: "scenario 1: test if adding an object triggers AddFunc",
			ns:   "ns-foo",
			gvr:  schema.GroupVersionResource{Group: "extensions", Version: "v1beta1", Resource: "deployments"},
			trigger: func(gvr schema.GroupVersionResource, ns string, fakeClient *fake.FakeDynamicClient, _ *unstructured.Unstructured) *unstructured.Unstructured {
				testObject := newUnstructured("extensions/v1beta1", "Deployment", "ns-foo", "name-foo")
				createdObj, err := fakeClient.Resource(gvr).Namespace(ns).Create(context.TODO(), testObject, metav1.CreateOptions{})
				if err != nil {
					t.Error(err)
				}
				return createdObj
			},
			handler: func(rcvCh chan<- *unstructured.Unstructured) *cache.ResourceEventHandlerFuncs {
				return &cache.ResourceEventHandlerFuncs{
					AddFunc: func(obj interface{}) {
						rcvCh <- obj.(*unstructured.Unstructured)
					},
				}
			},
		},

		// scenario 2
		{
			name:        "scenario 2: tests if updating an object triggers UpdateFunc",
			ns:          "ns-foo",
			gvr:         schema.GroupVersionResource{Group: "extensions", Version: "v1beta1", Resource: "deployments"},
			existingObj: newUnstructured("extensions/v1beta1", "Deployment", "ns-foo", "name-foo"),
			trigger: func(gvr schema.GroupVersionResource, ns string, fakeClient *fake.FakeDynamicClient, testObject *unstructured.Unstructured) *unstructured.Unstructured {
				testObject.Object["spec"] = "updatedName"
				updatedObj, err := fakeClient.Resource(gvr).Namespace(ns).Update(context.TODO(), testObject, metav1.UpdateOptions{})
				if err != nil {
					t.Error(err)
				}
				return updatedObj
			},
			handler: func(rcvCh chan<- *unstructured.Unstructured) *cache.ResourceEventHandlerFuncs {
				return &cache.ResourceEventHandlerFuncs{
					UpdateFunc: func(old, updated interface{}) {
						rcvCh <- updated.(*unstructured.Unstructured)
					},
				}
			},
		},

		// scenario 3
		{
			name:        "scenario 3: test if deleting an object triggers DeleteFunc",
			ns:          "ns-foo",
			gvr:         schema.GroupVersionResource{Group: "extensions", Version: "v1beta1", Resource: "deployments"},
			existingObj: newUnstructured("extensions/v1beta1", "Deployment", "ns-foo", "name-foo"),
			trigger: func(gvr schema.GroupVersionResource, ns string, fakeClient *fake.FakeDynamicClient, testObject *unstructured.Unstructured) *unstructured.Unstructured {
				err := fakeClient.Resource(gvr).Namespace(ns).Delete(context.TODO(), testObject.GetName(), metav1.DeleteOptions{})
				if err != nil {
					t.Error(err)
				}
				return testObject
			},
			handler: func(rcvCh chan<- *unstructured.Unstructured) *cache.ResourceEventHandlerFuncs {
				return &cache.ResourceEventHandlerFuncs{
					DeleteFunc: func(obj interface{}) {
						rcvCh <- obj.(*unstructured.Unstructured)
					},
				}
			},
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			// test data
			timeout := time.Duration(3 * time.Second)
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			scheme := runtime.NewScheme()
			informerReceiveObjectCh := make(chan *unstructured.Unstructured, 1)
			objs := []runtime.Object{}
			if ts.existingObj != nil {
				objs = append(objs, ts.existingObj)
			}
			// don't adjust the scheme to include deploymentlist. This is testing whether an informer can be created against using
			// a client that doesn't have a type registered in the scheme.
			gvrToListKind := map[schema.GroupVersionResource]string{
				{Group: "extensions", Version: "v1beta1", Resource: "deployments"}: "DeploymentList",
			}
			fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objs...)
			manager := NewSingleClusterInformerManager(fakeClient, 0, ctx.Done())
			// act
			manager.ForResource(ts.gvr, ts.handler(informerReceiveObjectCh))
			manager.Start()
			if synced := manager.WaitForCacheSync(); !synced[ts.gvr] {
				t.Errorf("informer for %s hasn't synced", ts.gvr)
			}

			testObject := ts.trigger(ts.gvr, ts.ns, fakeClient, ts.existingObj)
			select {
			case objFromInformer := <-informerReceiveObjectCh:
				if !equality.Semantic.DeepEqual(testObject, objFromInformer) {
					t.Fatalf("%v", diff.ObjectDiff(testObject, objFromInformer))
				}
			case <-ctx.Done():
				t.Errorf("tested informer haven't received an object, waited %v", timeout)
			}
		})
	}
}

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
			"spec": name,
		},
	}
}
