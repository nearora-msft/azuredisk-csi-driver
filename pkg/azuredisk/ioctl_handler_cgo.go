package azuredisk

// #include <fda_interface.h>
// #include <fda_rq_context.h>
import "C"
import "k8s.io/klog/v2"

func attach_to_host() {
	klog.Infof("Initiating attach with host")

	val := C.fda_vsc_init()

	activityId := ""
	vmId := ""
	blobURL := ""
	dsasKey := ""

	activityIdStr := C.CString(activityId)
	vmIdStr := C.CString(vmId)
	blobURLStr := C.CString(blobURL)
	DSASKeyStr := C.CString(dsasKey)

	result := C.fda_disk_attach(val, activityIdStr, C.uint(len(activityId)), vmIdStr, C.uint(len(vmId)), blobURLStr, C.uint(len(blobURL)), DSASKeyStr, C.uint(len(dsasKey)))

	if result == C.FDA_OP_SUCCEEDED {
		klog.Infof("The attach was successful")
	}
}

func detach() {

}
