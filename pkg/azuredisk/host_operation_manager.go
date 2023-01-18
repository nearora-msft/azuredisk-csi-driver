package azuredisk

// #include <fda_interface.h>
// #include <fda_rq_context.h>
//#include <stdlib.h>
import "C"
import (
	"errors"
	"unsafe"

	"github.com/google/uuid"
	"k8s.io/klog/v2"
)

func attachToHost(blobURL string, dsasKey string) (int, error) {
	klog.Infof("Initiating attach with host")

	ctx := C.fda_vsc_init()
	if ctx == nil {
		return -1, errors.New("couldn't initialize vsc")

	}
	defer C.fda_vsc_cleanup(ctx)

	activityId := uuid.New()
	vmId := ""

	activityIdStr := C.CString(activityId.String())
	defer C.free(unsafe.Pointer(activityIdStr))

	vmIdStr := C.CString(vmId)
	defer C.free(unsafe.Pointer(vmIdStr))

	blobURLStr := C.CString(blobURL)
	defer C.free(unsafe.Pointer(blobURLStr))

	dsasKeyStr := C.CString(dsasKey)
	defer C.free(unsafe.Pointer(dsasKeyStr))

	result := C.fda_disk_attach(ctx, activityIdStr, C.uint(len(activityId)), vmIdStr, C.uint(len(vmId)), blobURLStr, C.uint(len(blobURL)), dsasKeyStr, C.uint(len(dsasKey)))

	if result == C.FDA_OP_SUCCEEDED {
		klog.Infof("The attach was successful")
	}

	// Todo: Return the lun number
	return -1, nil
}

func detachFromHost(blobURL string, dsasKey string) error {
	klog.Infof("Initiating detach from host")

	ctx := C.fda_vsc_init()
	if ctx == nil {
		return errors.New("couldn't initialize vsc")
	}
	defer C.fda_vsc_cleanup(ctx)

	activityId := uuid.New()
	vmId := ""

	activityIdStr := C.CString(activityId.String())
	defer C.free(unsafe.Pointer(activityIdStr))

	vmIdStr := C.CString(vmId)
	defer C.free(unsafe.Pointer(vmIdStr))

	blobURLStr := C.CString(blobURL)
	defer C.free(unsafe.Pointer(blobURLStr))

	dsasKeyStr := C.CString(dsasKey)
	defer C.free(unsafe.Pointer(dsasKeyStr))

	result := C.fda_disk_detach(ctx, activityIdStr, C.uint(len(activityId)), vmIdStr, C.uint(len(vmId)), blobURLStr, C.uint(len(blobURL)), dsasKeyStr, C.uint(len(dsasKey)))

	if result == C.FDA_OP_SUCCEEDED {
		klog.Infof("The detach was successful")
	}

	return nil
}
