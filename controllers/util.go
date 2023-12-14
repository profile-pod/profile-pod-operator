package controllers

import (
	profilepodiov1alpha1 "github.com/profile-pod/profile-pod-operator/api/v1alpha1"
)

func labelsForPodfalme(cr *profilepodiov1alpha1.PodFlame) map[string]string {
	return map[string]string{
		"app.kubernetes.io/instance":   string(cr.UID),
		"app.kubernetes.io/managed-by": "profile-pod-operator",
	}
}
