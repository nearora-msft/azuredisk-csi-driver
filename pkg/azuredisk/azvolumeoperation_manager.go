package azuredisk

import (
	"context"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/azuredisk-csi-driver/pkg/apis/azuredisk/v1alpha1"
	azdisk "sigs.k8s.io/azuredisk-csi-driver/pkg/apis/client/clientset/versioned"
	azdiskinformers "sigs.k8s.io/azuredisk-csi-driver/pkg/apis/client/informers/externalversions"
	"sigs.k8s.io/azuredisk-csi-driver/pkg/azureconstants"
	consts "sigs.k8s.io/azuredisk-csi-driver/pkg/azureconstants"
)

type AzVolumeOperationManager struct {
	clientSet *azdisk.Clientset
	nodeID    string
}

func NewAzVolumeOperationManager(clientSet *azdisk.Clientset, nodeId string) *AzVolumeOperationManager {
	return &AzVolumeOperationManager{
		clientSet: clientSet,
		nodeID:    nodeId,
	}
}

func (mgr *AzVolumeOperationManager) Init(ctx context.Context) {
	klog.V(2).Info("Initiating AzVolumeOPeration infomers")
	azurediskInformerFactory := azdiskinformers.NewSharedInformerFactoryWithOptions(mgr.clientSet, time.Duration(30)*time.Second, azdiskinformers.WithTweakListOptions(func(lo *v1.ListOptions) {
		lo.LabelSelector = labels.Set{consts.VolumeOperationManagedBy: mgr.nodeID}.AsSelector().String()
	}))

	azVolumeOperationInformer := azurediskInformerFactory.Disk().V1alpha1().AzVolumeOperations()

	azVolumeOperationInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    mgr.onAzVolumeOperationAdd,
			UpdateFunc: mgr.onAzVolumeOperationUpdate,
		})

	go azurediskInformerFactory.Start(ctx.Done())

	if !cache.WaitForCacheSync(
		ctx.Done(),
		azVolumeOperationInformer.Informer().HasSynced,
	) {
		klog.Fatal("failed to sync and populate the cache for AzVolumeoperation informer")
		os.Exit(1)
	}
}

func (mgr *AzVolumeOperationManager) onAzVolumeOperationAdd(obj interface{}) {
	azVolumeOperation := obj.(*v1alpha1.AzVolumeOperation)

	klog.V(2).Infof("Initiating attach for volume %s", azVolumeOperation.Spec.DiskURI)

	//Todo: Make a call to host to attach

	copyForUpdate := azVolumeOperation.DeepCopy()
	copyForUpdate.Status = v1alpha1.AzVolumeOperationStatus{
		// Todo: Remove the dummy lun value
		Lun:   "0",
		State: v1alpha1.VolumeAttached,
	}

	_, err := mgr.clientSet.DiskV1alpha1().AzVolumeOperations(azureconstants.DefaultCustomObjectNamespace).UpdateStatus(context.Background(), copyForUpdate, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("failed to update AzvolumeOperation after attach with error: %v", err)
	}
}

func (mgr *AzVolumeOperationManager) onAzVolumeOperationUpdate(oldObj interface{}, newObj interface{}) {
	azVolumeOperationNew := newObj.(*v1alpha1.AzVolumeOperation)
	if azVolumeOperationNew.Spec.RequestedOperation == v1alpha1.Detach && azVolumeOperationNew.Status.State == v1alpha1.VolumeAttached {
		klog.V(2).Infof("Initiating detach for volume %s", azVolumeOperationNew.Spec.DiskURI)
		//Todo: Make a call to host to detach

		copyForUpdate := azVolumeOperationNew.DeepCopy()
		copyForUpdate.Status.State = v1alpha1.VolumeDetached
		if _, err := mgr.clientSet.DiskV1alpha1().AzVolumeOperations(azureconstants.DefaultCustomObjectNamespace).UpdateStatus(context.Background(), copyForUpdate, metav1.UpdateOptions{}); err != nil {
			klog.Errorf("failed to update AzvolumeOperation after detach with error: %v", err)
		}
	}

}
