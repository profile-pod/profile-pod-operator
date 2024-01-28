package controllers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"

	profilepodiov1alpha1 "github.com/profile-pod/profile-pod-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ContainerName = "pod-profiler"
	AgentImageKey = "AGENT_IMAGE"
	AgentImageDefault = "pp:v1"
)

func (reconciler *PodFlameReconciler) definePod(podflame *profilepodiov1alpha1.PodFlame, namespace, podName string, ctx context.Context) (*corev1.Pod, error) {
	var volumeName = "runtime-path"
	targetPod, err := GetTargetPod(reconciler.Clientset, podflame.Spec.TargetPod, podflame.Namespace, ctx)
	if err != nil {
		return nil, err
	}
	targetContainerName, err := getContainerName(targetPod, podflame)
	if err != nil {
		return nil, err
	}
	runtime, targetContainerId, err := GetContainerDetailes(targetContainerName, targetPod)
	if err != nil {
		return nil, err
	}
	hostpath, err := GetContainerRuntimePath(runtime)
	if err != nil {
		return nil, err
	}
	args := []string{
		string(targetPod.UID), targetContainerName, targetContainerId, runtime,
		podflame.Spec.Duration, podflame.Spec.Event,
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    labelsForPodfalme(podflame),
			Annotations: map[string]string{
				"sidecar.istio.io/inject": "false",
				"profilepod.io/name":      podflame.Name,
				"profilepod.io/namespace": podflame.Namespace,
			},
		},
		Spec: corev1.PodSpec{
			HostPID:       true,
			RestartPolicy: corev1.RestartPolicyNever,
			NodeName:      targetPod.Spec.NodeName,
			Volumes: []corev1.Volume{
				{
					Name: volumeName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostpath,
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					ImagePullPolicy: corev1.PullIfNotPresent,
					Name:            ContainerName,
					Image:           GetAgentImage(),
					Command:         []string{"/app/agent"},
					Args:            args,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      volumeName,
							MountPath: "/runtimepath",
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"ALL"},
						},
					},
				},
			},
		},
	}
	// if err := ctrl.SetControllerReference(podflame, pod, reconciler.Scheme); err != nil {
	// 	return nil, err
	// }
	return pod, nil
}

func GetAgentImage() string {
	image, found := os.LookupEnv(AgentImageKey)
	if !found {
		image = AgentImageDefault
	}
	return image;
}

func (reconciler *PodFlameReconciler) reconcilePod(ctx context.Context, podflame *profilepodiov1alpha1.PodFlame) (ctrl.Result, error) {
	var namespace = reconciler.OperatorNamesapce
	var podName = podflame.Namespace + "-" + podflame.Name
	log := log.FromContext(ctx)
	var pod = &corev1.Pod{}
	err := reconciler.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespace}, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if podflame.Status.Failed == "" && podflame.Status.FlameGraph == "" {
				log.Info("Pod resource " + podName + " not found. Creating or re-creating pod")
				podDefinition, err := reconciler.definePod(podflame, namespace, podName, ctx)
				if err != nil {
					log.Info("Failed to create Pod definition. Re-running reconcile.")
					return ctrl.Result{}, err
				}
				err = reconciler.Create(ctx, podDefinition)
				if err != nil {
					log.Info("Failed to create Pod resource. Re-running reconcile.")
					return ctrl.Result{}, err
				}
			}
		} else {
			log.Info("Failed to get Pod resource " + podName + ". Re-running reconcile.")
			return ctrl.Result{}, err
		}
	} else {
		if podflame.Status.Failed != "" || podflame.Status.FlameGraph != "" {
			log.Info("Pod resource " + podName + " found after profile finished. deleting Pod")
			err = reconciler.Clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
			if err != nil {
				log.Info("Failed to delete pod resource. Re-running reconcile.")
				return ctrl.Result{}, err
			}
		} else {
			switch pod.Status.Phase {
			case corev1.PodFailed:
				logs, err := getPodLogs(reconciler.Clientset, namespace, pod.Name)
				if err != nil {
					log.Info("Failed to get logs from failed profile pod. Re-running reconcile.")
					return ctrl.Result{}, err
				}
				podflame.Status.Failed = logs
				if err = reconciler.Status().Update(ctx, podflame); err != nil {
					log.Error(err, "Failed to update podflame status")
					return ctrl.Result{}, err
				}
			case corev1.PodSucceeded:
				logs, err := getPodLogs(reconciler.Clientset, namespace, pod.Name)
				if err != nil {
					log.Info("Failed to get logs from succeeded profile pod. Re-running reconcile.")
					return ctrl.Result{}, err
				}
				podflame.Status.FlameGraph = logs
				if err = reconciler.Status().Update(ctx, podflame); err != nil {
					log.Error(err, "Failed to update podflame status")
					return ctrl.Result{}, err
				}
			case corev1.PodRunning:
				log.Info("Profiler pod is running")
			default:
				log.Info("Profiler pod is init")
			}
		}
	}

	return ctrl.Result{}, nil
}

func getPodLogs(clientset *kubernetes.Clientset, namespace, podName string) (string, error) {
	podLogs, err := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{}).Stream(context.TODO())
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	// Read the logs from the stream
	logs := make([]byte, 4096)
	readLen, err := podLogs.Read(logs)
	if err != nil {
		return "", err
	}

	return string(logs[:readLen]), nil
}

func GetTargetPod(clientset *kubernetes.Clientset, podName, namespace string, ctx context.Context) (*corev1.Pod, error) {
	podObject, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return podObject, nil
}

func getContainerName(pod *corev1.Pod, podflame *profilepodiov1alpha1.PodFlame) (string, error) {
	if len(pod.Spec.Containers) != 1 {
		var containerNames []string
		for _, container := range pod.Spec.Containers {
			if container.Name == podflame.Spec.ContainerName {
				return container.Name, nil // Found given container
			}

			containerNames = append(containerNames, container.Name)
		}

		return "", fmt.Errorf("Could not determine container. please specify one of %v", containerNames)
	}

	return pod.Spec.Containers[0].Name, nil
}

func GetContainerDetailes(containerName string, pod *corev1.Pod) (string, string, error) {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name == containerName {
			if containerStatus.State.Running == nil {
				return "", "", errors.New("Container is not running: " + containerName)
			}
			re := regexp.MustCompile(`^([^:]+)://([^/]+)$`)
			matches := re.FindStringSubmatch(containerStatus.ContainerID)
			return matches[1], matches[2], nil
		}
	}

	return "", "", errors.New("Could not find container id for " + containerName)
}

func GetContainerRuntimePath(runtime string) (string, error) {
	switch runtime {
	case "docker":
		return "/var/lib/docker", nil
	case "containerd":
		return "/run/containerd", nil
	default:
		return "", errors.New("Unknown container runtime " + runtime)
	}
}
