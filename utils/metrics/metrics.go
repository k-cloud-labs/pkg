package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

type ErrorType string

const (
	ErrorTypeUnknown        ErrorType = "unknown"
	ErrorTypeCueExecute     ErrorType = "cue_execute_error"
	ErrorTypeOriginExecute  ErrorType = "cue_origin_error"
	ErrTypePrepareCueParams ErrorType = "prepare_cue_params_error"

	SubSystemName = "kcloudlabs"
)

var (
	policyTotalNumber = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: SubSystemName,
			Name:      "policy_total_number",
			Help:      "Total number of added policies",
		},
		[]string{"policy_type"},
	)
	overridePolicyMatchedCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubSystemName,
			Name:      "override_policy_matched_count",
			Help:      "The number of resources changes matched with override policies",
		},
		[]string{"name", "resource_type"},
	)

	overridePolicyOverrideCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubSystemName,
			Name:      "override_policy_override_count",
			Help:      "The number of resources changes override by override policies",
		},
		[]string{"name", "resource_type"},
	)

	validatePolicyMatchedCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubSystemName,
			Name:      "validate_policy_matched_count",
			Help:      "The number of resources changes matched with validate policies",
		},
		[]string{"name", "resource_type"},
	)

	validatePolicyRejectCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubSystemName,
			Name:      "validate_policy_reject_count",
			Help:      "The number of resources changes rejected by validate policies",
		},
		[]string{"name", "resource_type"},
	)

	policyErrorCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubSystemName,
			Name:      "policy_error_count",
			Help:      "Count of error when policy engine handle policy",
		},
		[]string{"name", "resource_type", "error_type"},
	)

	policySuccessCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubSystemName,
			Name:      "policy_success_count",
			Help:      "Count of success when policy engine handle policy",
		},
		[]string{"name", "resource_type"},
	)

	resourceSyncErrorCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubSystemName,
			Name:      "resource_sync_error_count",
			Help:      "Count of error when sync resource to cache",
		},
		[]string{"resource_type"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		policyTotalNumber,
		overridePolicyMatchedCount,
		overridePolicyOverrideCount,
		validatePolicyMatchedCount,
		validatePolicyRejectCount,
		policyErrorCount,
		resourceSyncErrorCount,
	)
}

func IncrPolicy(policyType string) {
	policyTotalNumber.WithLabelValues(policyType).Inc()
}

func DecPolicy(policyType string) {
	policyTotalNumber.WithLabelValues(policyType).Dec()
}

func SetPolicyNumber(policyType string, n int64) {
	policyTotalNumber.WithLabelValues(policyType).Set(float64(n))
}

func OverridePolicyMatched(policyName string, resourceGVK schema.GroupVersionKind) {
	overridePolicyMatchedCount.WithLabelValues(policyName,
		fmt.Sprintf("%s/%s/%s", resourceGVK.Group, resourceGVK.Version, resourceGVK.Kind)).Inc()
}

func OverridePolicyOverride(policyName string, resourceGVK schema.GroupVersionKind) {
	overridePolicyOverrideCount.WithLabelValues(policyName,
		fmt.Sprintf("%s/%s/%s", resourceGVK.Group, resourceGVK.Version, resourceGVK.Kind)).Inc()
}

func ValidatePolicyMatched(policyName string, resourceGVK schema.GroupVersionKind) {
	validatePolicyMatchedCount.WithLabelValues(policyName,
		fmt.Sprintf("%s/%s/%s", resourceGVK.Group, resourceGVK.Version, resourceGVK.Kind)).Inc()
}

func ValidatePolicyReject(policyName string, resourceGVK schema.GroupVersionKind) {
	validatePolicyRejectCount.WithLabelValues(policyName,
		fmt.Sprintf("%s/%s/%s", resourceGVK.Group, resourceGVK.Version, resourceGVK.Kind)).Inc()
}

func PolicyGotError(policyName string, resourceGVK schema.GroupVersionKind, errorType ErrorType) {
	policyErrorCount.WithLabelValues(policyName,
		fmt.Sprintf("%s/%s/%s", resourceGVK.Group, resourceGVK.Version, resourceGVK.Kind),
		string(errorType)).Inc()
}

func PolicySuccess(policyName string, resourceGVK schema.GroupVersionKind) {
	policySuccessCount.WithLabelValues(policyName,
		fmt.Sprintf("%s/%s/%s", resourceGVK.Group, resourceGVK.Version, resourceGVK.Kind)).Inc()
}

func SyncResourceError(resourceGVK schema.GroupVersionKind) {
	resourceSyncErrorCount.WithLabelValues(
		fmt.Sprintf("%s/%s/%s", resourceGVK.Group, resourceGVK.Version, resourceGVK.Kind),
	).Inc()
}
