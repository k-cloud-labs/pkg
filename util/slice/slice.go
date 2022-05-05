package slice

import (
	admissionv1 "k8s.io/api/admission/v1"
)

func Exists(items []admissionv1.Operation, pattern admissionv1.Operation) bool {
	for _, item := range items {
		if item == pattern {
			return true
		}
	}

	return false
}
