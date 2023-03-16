package azuredisk

// #include <fda_interface.h>
// #include <fda_rq_context.h>
//#include <stdlib.h>
import "C"
import (
	"errors"
	"unsafe"

	"k8s.io/klog/v2"
	"sigs.k8s.io/azuredisk-csi-driver/pkg/apis/azuredisk/v1alpha1"
)

func hostAttachDetach(blobURL string, lun int, requestedOperation v1alpha1.RequestedOperation) error {

	ctx := C.fda_vsc_init()
	if ctx == nil {
		return errors.New("couldn't initialize vsc")

	}
	defer C.fda_vsc_cleanup(ctx)

	blobURLStr := C.CString(blobURL)
	defer C.free(unsafe.Pointer(blobURLStr))

	var result C.int
	if requestedOperation == v1alpha1.Attach {
		result = C.fda_disk_attach(ctx, blobURLStr, C.uint(len(blobURL)), C.uint(lun))
	} else {
		result = C.fda_disk_detach(ctx, blobURLStr, C.uint(len(blobURL)), C.uint(lun))
	}

	if result == C.FDA_OP_SUCCEEDED {
		klog.Infof("The %s operation was successful", requestedOperation)
	}

	return nil
}

// func attachToHost(blobURL string, lun int) error {
// 	klog.Infof("Initiating attach with host")

// 	ctx := C.fda_vsc_init()
// 	if ctx == nil {
// 		return errors.New("couldn't initialize vsc")

// 	}
// 	defer C.fda_vsc_cleanup(ctx)

// 	blobURLStr := C.CString(blobURL)
// 	defer C.free(unsafe.Pointer(blobURLStr))

// 	result := C.fda_disk_attach(ctx, blobURLStr, C.uint32(len(blobURLStr)), C.uint32(lun))

// 	//result := C.fda_disk_attach(ctx, activityIdStr, C.uint(len(activityId)), vmIdStr, C.uint(len(vmId)), blobURLStr, C.uint(len(blobURL)), dsasKeyStr, C.uint(len(dsasKey)))

// 	if result == C.FDA_OP_SUCCEEDED {
// 		klog.Infof("The attach was successful")
// 	}

// 	// Todo: Return the lun number
// 	return -1, nil
// }

// func detachFromHost(blobURL string, lun int) error {
// 	klog.Infof("Initiating detach from host")

// 	ctx := C.fda_vsc_init()
// 	if ctx == nil {
// 		return errors.New("couldn't initialize vsc")
// 	}
// 	defer C.fda_vsc_cleanup(ctx)

// 	activityIdStr := C.CString(activityId.String())
// 	defer C.free(unsafe.Pointer(activityIdStr))

// 	blobURLStr := C.CString(blobURL)
// 	defer C.free(unsafe.Pointer(blobURLStr))

// 	result := C.fda_disk_detach(ctx)
// 	// result := C.fda_disk_detach(ctx, activityIdStr, C.uint(len(activityId)), vmIdStr, C.uint(len(vmId)), blobURLStr, C.uint(len(blobURL)), dsasKeyStr, C.uint(len(dsasKey)))

// 	if result == C.FDA_OP_SUCCEEDED {
// 		klog.Infof("The detach was successful")
// 	}

// 	return nil
// }
