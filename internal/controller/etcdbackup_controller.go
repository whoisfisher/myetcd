/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	etcdv1alpha1 "xiaoming.com/myetcd/api/v1alpha1"
)

// EtcdBackupReconciler reconciles a EtcdBackup object
type EtcdBackupReconciler struct {
	client.Client
	Log         logr.Logger
	BackupImage string
	Scheme      *runtime.Scheme
}

type backupState struct {
	backup  *etcdv1alpha1.EtcdBackup
	actual  *backupStateContainer
	desired *backupStateContainer
}

type backupStateContainer struct {
	pod *corev1.Pod
}

func (r *EtcdBackupReconciler) setStateActual(ctx context.Context, state *backupState) error {
	var actual backupStateContainer
	key := client.ObjectKey{
		Name:      state.backup.Name,
		Namespace: state.backup.Namespace,
	}

	actual.pod = &corev1.Pod{}

	if err := r.Get(ctx, key, actual.pod); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("Getting pod error: %s", err)
		}
		actual.pod = nil
	}
	state.actual = &actual
	return nil
}

func (r *EtcdBackupReconciler) setStateDesired(state *backupState) error {
	var desired backupStateContainer
	pod, err := podForBackup(state.backup, r.BackupImage)
	if err != nil {
		return fmt.Errorf("computing pod for backup error: %q", err)
	}
	if err := controllerutil.SetControllerReference(state.backup, pod, r.Scheme); err != nil {
		return fmt.Errorf("setting pod controller reference error: %q", err)
	}
	desired.pod = pod
	state.desired = &desired
	return nil
}

func (r *EtcdBackupReconciler) getState(ctx context.Context, req ctrl.Request) (*backupState, error) {
	var state backupState
	state.backup = &etcdv1alpha1.EtcdBackup{}
	if err := r.Get(ctx, req.NamespacedName, state.backup); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return nil, fmt.Errorf("getting backup error: %q", err)
		}
		state.backup = nil
		return &state, nil
	}

	if err := r.setStateActual(ctx, &state); err != nil {
		return nil, fmt.Errorf("setting actual state error: %s", err)
	}

	if err := r.setStateDesired(&state); err != nil {
		return nil, fmt.Errorf("setting desired state error :%s", err)
	}

	return &state, nil
}

func podForBackup(backup *etcdv1alpha1.EtcdBackup, image string) (*corev1.Pod, error) {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: backup.Namespace,
			Name:      backup.Name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "backup-agent",
					Image: image,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("50Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("50Mi"),
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}, nil
}

// +kubebuilder:rbac:groups=etcd.xiaoming.com,resources=etcdbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=etcd.xiaoming.com,resources=etcdbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=etcd.xiaoming.com,resources=etcdbackups/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EtcdBackup object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *EtcdBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("etcdbackup", req.NamespacedName)

	state, err := r.getState(ctx, req)
	if err != nil {
		return ctrl.Result{}, err
	}

	var action Action

	switch {
	case state.backup == nil:
		log.Info("Backup Object not found. Ignoring.")
	case !state.backup.DeletionTimestamp.IsZero():
		log.Info("Backup Object has been deleted. Ignoring.")
	case state.backup.Status.Phase == "":
		log.Info("Backup Starting. Updating status.")
		newBackup := state.backup.DeepCopy()
		newBackup.Status.Phase = etcdv1alpha1.EtcdBackupPhaseBackingUp
		action = &PatchStatus{client: r.Client, original: state.backup, new: newBackup}
	case state.backup.Status.Phase == etcdv1alpha1.EtcdbackupPhaseFailed:
		log.Info("Backup has failed. Ignoring.")
	case state.backup.Status.Phase == etcdv1alpha1.EtcdBackupPhaseCompleted:
		log.Info("Backup has completed. Ignoring.")
	case state.actual.pod == nil:
		log.Info("Backup Pod failed. Updating status.")
		action = &CreateObject{client: r.Client, obj: state.desired.pod}
	case state.actual.pod.Status.Phase == corev1.PodFailed:
		log.Info("Backup Pod failed. Updating status.")
		newBackup := state.backup.DeepCopy()
		newBackup.Status.Phase = etcdv1alpha1.EtcdbackupPhaseFailed
		action = &PatchStatus{client: r.Client, original: state.backup, new: newBackup}
	case state.actual.pod.Status.Phase == corev1.PodSucceeded:
		log.Info("Backup Pod succeeded. Updating status.")
		newBackup := state.backup.DeepCopy()
		newBackup.Status.Phase = etcdv1alpha1.EtcdBackupPhaseCompleted
		action = &PatchStatus{client: r.Client, original: state.backup, new: newBackup}
	}

	if action != nil {
		if err := action.Execute(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("executing action error: %q", err)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EtcdBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&etcdv1alpha1.EtcdBackup{}).
		Named("etcdbackup").
		Complete(r)
}
