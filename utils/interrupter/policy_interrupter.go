package interrupter

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	jsonpatch "github.com/evanphx/json-patch"
	"golang.org/x/sync/errgroup"
	jsonpatchv2 "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
)

// PolicyInterrupterManager manage multi PolicyInterrupter and decide which one to use by gvk.
type PolicyInterrupterManager interface {
	PolicyInterrupter
	// AddInterrupter add a PolicyInterrupter to manager,
	//  it will replace interrupter if already add with same gvk.s
	AddInterrupter(gvk schema.GroupVersionKind, pi PolicyInterrupter)
}

// PolicyInterrupter defines interrupt process for policy change
// It validate and mutate policy.
type PolicyInterrupter interface {
	// OnMutating called on "/mutating" api to complete policy
	// return nil means obj is not defined policy
	OnMutating(obj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) ([]jsonpatchv2.JsonPatchOperation, error)
	// OnValidating called on "/validating" api to validate policy
	// return nil means obj is not defined policy or no invalid field
	OnValidating(obj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) error
	// OnStartUp called when webhook process initialize
	// return error if initial phase get any error
	OnStartUp() error
}

type policyInterrupterImpl struct {
	interrupters sync.Map
}

func (p *policyInterrupterImpl) OnMutating(obj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) ([]jsonpatchv2.JsonPatchOperation, error) {
	interrupter := p.getInterrupter(obj)
	if interrupter == nil {
		return nil, nil
	}

	patches, err := interrupter.OnMutating(obj, oldObj, operation)
	if err != nil {
		return nil, err
	}

	if len(patches) > 0 {
		if err = applyJSONPatch(obj, patches); err != nil {
			klog.ErrorS(err, "apply json patch error",
				"kind", obj.GetKind(), "namespace", obj.GetNamespace(), "name", obj.GetName())
			return nil, err
		}
	}

	return patches, nil
}

func (p *policyInterrupterImpl) OnValidating(obj, oldObj *unstructured.Unstructured, operation admissionv1.Operation) error {
	if interrupter := p.getInterrupter(obj); interrupter != nil {
		return interrupter.OnValidating(obj, oldObj, operation)
	}

	return nil
}

func (p *policyInterrupterImpl) isKnownPolicy(obj *unstructured.Unstructured) bool {
	group := strings.Split(obj.GetAPIVersion(), "/")[0]
	return group == policyv1alpha1.SchemeGroupVersion.Group
}

func (p *policyInterrupterImpl) getInterrupter(obj *unstructured.Unstructured) PolicyInterrupter {
	if !p.isKnownPolicy(obj) {
		klog.V(5).InfoS("unknown policy", "gvk", obj.GroupVersionKind())
		return nil
	}

	i, ok := p.interrupters.Load(obj.GroupVersionKind())
	if ok {
		klog.V(4).InfoS("sub interrupter found", "gvk", obj.GroupVersionKind())
		return i.(PolicyInterrupter)
	}

	return nil
}

func (p *policyInterrupterImpl) OnStartUp() error {
	eg, _ := errgroup.WithContext(context.Background())
	p.interrupters.Range(func(key, value any) bool {
		interrupter := value.(PolicyInterrupter)
		eg.Go(interrupter.OnStartUp)
		return true
	})

	return eg.Wait()
}

func (p *policyInterrupterImpl) AddInterrupter(gvk schema.GroupVersionKind, pi PolicyInterrupter) {
	p.interrupters.Store(gvk, pi)
}

func NewPolicyInterrupterManager() PolicyInterrupterManager {
	return &policyInterrupterImpl{
		interrupters: sync.Map{},
	}
}

func applyJSONPatch(obj *unstructured.Unstructured, overrides []jsonpatchv2.JsonPatchOperation) error {
	jsonPatchBytes, err := json.Marshal(overrides)
	if err != nil {
		return err
	}

	patch, err := jsonpatch.DecodePatch(jsonPatchBytes)
	if err != nil {
		return err
	}

	objectJSONBytes, err := obj.MarshalJSON()
	if err != nil {
		return err
	}

	patchedObjectJSONBytes, err := patch.Apply(objectJSONBytes)
	if err != nil {
		return err
	}

	err = obj.UnmarshalJSON(patchedObjectJSONBytes)
	return err
}
