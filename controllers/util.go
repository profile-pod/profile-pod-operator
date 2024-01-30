package controllers

import (
	profilepodiov1alpha1 "github.com/profile-pod/profile-pod-operator/api/v1alpha1"
	"github.com/profile-pod/profile-pod-operator/controllers/constants"
)

func labelsForPodfalme(cr *profilepodiov1alpha1.PodFlame) map[string]string {
	return map[string]string{
		constants.Instance:   string(cr.UID),
		constants.ManagedBy: constants.OperatorName,
	}
}
