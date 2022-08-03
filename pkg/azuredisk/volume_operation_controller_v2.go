package azuredisk

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	clientset "sigs.k8s.io/azuredisk-csi-driver/pkg/apis/azuredisk/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type VolumeOperationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *VolumeOperationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var vop clientset.AzVolumeOperation
	if err := r.Get(ctx, req.NamespacedName, &vop); err != nil {
		return ctrl.Result{Requeue: false}, client.IgnoreNotFound(err)
	}

	if vop.DeletionTimestamp.IsZero() {
		klog.Infof("The CRI was deleted")
		// Add finalizer
		// TODO: Add logic to attach

	} else {
		klog.Infof("The CRI was added")
		// Remove finalizer
		// TODO: Add logic to detach
	}

	return ctrl.Result{Requeue: false}, nil
}

func (r *VolumeOperationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clientset.AzVolumeOperation{}).
		WithEventFilter(definePredicates()).
		Complete(r)
}

func definePredicates() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// func NewVolumeOperationReconciler(mgr ctrl.Manager, storageClassName string) error {
// 	r := VolumeOperationReconciler{
// 		Client: mgr.GetClient(),
// 		Scheme: mgr.GetScheme(),
// 	}

// }
