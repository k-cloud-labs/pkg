package converter

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	policyv1alpha1 "github.com/k-cloud-labs/pkg/apis/policy/v1alpha1"
	"github.com/k-cloud-labs/pkg/utils"
)

// ConvertToPod converts a Pod object from unstructured to typed.
func ConvertToPod(obj *unstructured.Unstructured) (*corev1.Pod, error) {
	typedObj := &corev1.Pod{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToNode converts a Node object from unstructured to typed.
func ConvertToNode(obj *unstructured.Unstructured) (*corev1.Node, error) {
	typedObj := &corev1.Node{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToReplicaSet converts a ReplicaSet object from unstructured to typed.
func ConvertToReplicaSet(obj *unstructured.Unstructured) (*appsv1.ReplicaSet, error) {
	typedObj := &appsv1.ReplicaSet{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToDeployment converts a Deployment object from unstructured to typed.
func ConvertToDeployment(obj *unstructured.Unstructured) (*appsv1.Deployment, error) {
	typedObj := &appsv1.Deployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToDaemonSet converts a DaemonSet object from unstructured to typed.
func ConvertToDaemonSet(obj *unstructured.Unstructured) (*appsv1.DaemonSet, error) {
	typedObj := &appsv1.DaemonSet{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToStatefulSet converts a StatefulSet object from unstructured to typed.
func ConvertToStatefulSet(obj *unstructured.Unstructured) (*appsv1.StatefulSet, error) {
	typedObj := &appsv1.StatefulSet{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToJob converts a Job object from unstructured to typed.
func ConvertToJob(obj *unstructured.Unstructured) (*batchv1.Job, error) {
	typedObj := &batchv1.Job{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToEndpointSlice converts a EndpointSlice object from unstructured to typed.
func ConvertToEndpointSlice(obj *unstructured.Unstructured) (*discoveryv1.EndpointSlice, error) {
	typedObj := &discoveryv1.EndpointSlice{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToService converts a Service object from unstructured to typed.
func ConvertToService(obj *unstructured.Unstructured) (*corev1.Service, error) {
	typedObj := &corev1.Service{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToIngress converts a Service object from unstructured to typed.
func ConvertToIngress(obj *unstructured.Unstructured) (*extensionsv1beta1.Ingress, error) {
	typedObj := &extensionsv1beta1.Ingress{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToClusterValidatePolicy converts a ClusterValidatePolicy Object from unstructured to typed
func ConvertToClusterValidatePolicy(obj *unstructured.Unstructured) (*policyv1alpha1.ClusterValidatePolicy, error) {
	typedObj := &policyv1alpha1.ClusterValidatePolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToClusterOverridePolicy converts a ClusterOverridePolicy Object from unstructured to typed
func ConvertToClusterOverridePolicy(obj *unstructured.Unstructured) (*policyv1alpha1.ClusterOverridePolicy, error) {
	typedObj := &policyv1alpha1.ClusterOverridePolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToOverridePolicy converts a Override Object from unstructured to typed
func ConvertToOverridePolicy(obj *unstructured.Unstructured) (*policyv1alpha1.OverridePolicy, error) {
	typedObj := &policyv1alpha1.OverridePolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ApplyReplica applies the Replica value for the specific field.
func ApplyReplica(workload *unstructured.Unstructured, desireReplica int64, field string) error {
	_, ok, err := unstructured.NestedInt64(workload.Object, utils.SpecField, field)
	if err != nil {
		return err
	}
	if ok {
		err := unstructured.SetNestedField(workload.Object, desireReplica, utils.SpecField, field)
		if err != nil {
			return err
		}
	}
	return nil
}

// ToUnstructured converts a typed object to an unstructured object.
func ToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	uncastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: uncastObj}, nil
}
