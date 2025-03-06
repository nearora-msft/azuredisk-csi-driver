//go:build azurediskv2
// +build azurediskv2

/*
Copyright 2017 The Kubernetes Authors.

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

package azuredisk

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/container-storage-interface/spec/lib/go/csi"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	volerr "k8s.io/cloud-provider/volume/errors"
	"k8s.io/klog/v2"

	consts "sigs.k8s.io/azuredisk-csi-driver/pkg/azureconstants"
	"sigs.k8s.io/azuredisk-csi-driver/pkg/azureutils"
	"sigs.k8s.io/azuredisk-csi-driver/pkg/optimization"
	volumehelper "sigs.k8s.io/azuredisk-csi-driver/pkg/util"
	"sigs.k8s.io/cloud-provider-azure/pkg/metrics"
	azure "sigs.k8s.io/cloud-provider-azure/pkg/provider"
)

// CreateVolume provisions an azure disk
func (d *DriverV2) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if err := d.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		klog.Errorf("invalid create volume req: %v", req)
		return nil, err
	}

	params := make(map[string]string, len(req.GetParameters())+len(req.GetMutableParameters()))
	for k, v := range req.GetParameters() {
		params[k] = v
	}
	for k, v := range req.GetMutableParameters() {
		params[k] = v
	}
	diskParams, err := azureutils.ParseDiskParameters(params)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Failed parsing disk parameters: %v", err)
	}

	name := req.GetName()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Name must be provided")
	}
	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Volume capabilities must be provided")
	}

	if err := azureutils.IsValidVolumeCapabilities(volCaps, diskParams.MaxShares); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	isAdvancedPerfProfile := strings.EqualFold(diskParams.PerfProfile, consts.PerfProfileAdvanced)
	// If perfProfile is set to advanced and no/invalid device settings are provided, fail the request
	if d.getPerfOptimizationEnabled() && isAdvancedPerfProfile {
		if err := optimization.AreDeviceSettingsValid(consts.DummyBlockDevicePathLinux, diskParams.DeviceSettings); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	if acquired := d.volumeLocks.TryAcquire(name); !acquired {
		return nil, status.Errorf(codes.Aborted, volumeOperationAlreadyExistsFmt, name)
	}
	defer d.volumeLocks.Release(name)

	capacityBytes := req.GetCapacityRange().GetRequiredBytes()
	volSizeBytes := int64(capacityBytes)
	requestGiB := int(volumehelper.RoundUpGiB(volSizeBytes))

	if diskParams.PerformancePlus != nil && *diskParams.PerformancePlus && requestGiB < consts.PerformancePlusMinimumDiskSizeGiB {
		klog.Warningf("using PerformancePlus, increasing requested disk size from %vGiB to %vGiB (minimal size for PerformancePlus feature)", requestGiB, consts.PerformancePlusMinimumDiskSizeGiB)
		requestGiB = consts.PerformancePlusMinimumDiskSizeGiB
	}
	if requestGiB < consts.MinimumDiskSizeGiB {
		klog.Infof("increasing requested disk size from %vGiB to %vGiB (minimal disk size)", requestGiB, consts.MinimumDiskSizeGiB)
		requestGiB = consts.MinimumDiskSizeGiB
	}

	maxVolSize := int(volumehelper.RoundUpGiB(req.GetCapacityRange().GetLimitBytes()))
	if (maxVolSize > 0) && (maxVolSize < requestGiB) {
		return nil, status.Error(codes.InvalidArgument, "After round-up, volume size exceeds the limit specified")
	}

	if azureutils.IsAzureStackCloud(d.cloud.Config.Cloud, d.cloud.Config.DisableAzureStackCloud) {
		if diskParams.MaxShares > 1 {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid maxShares value: %d as Azure Stack does not support shared disk.", diskParams.MaxShares))
		}
	}

	if diskParams.DiskName == "" {
		diskParams.DiskName = name
	}
	diskParams.DiskName = azureutils.CreateValidDiskName(diskParams.DiskName)

	if diskParams.ResourceGroup == "" {
		diskParams.ResourceGroup = d.cloud.ResourceGroup
	}

	// normalize values
	skuName, err := azureutils.NormalizeStorageAccountType(diskParams.AccountType, d.cloud.Config.Cloud, d.cloud.Config.DisableAzureStackCloud)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if _, err := azureutils.NormalizeCachingMode(diskParams.CachingMode); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if skuName == armcompute.DiskStorageAccountTypesPremiumV2LRS {
		// PremiumV2LRS only supports None caching mode
		azureutils.SetKeyValueInMap(diskParams.VolumeContext, consts.CachingModeField, string(v1.AzureDataDiskCachingNone))
	}

	if err := azureutils.ValidateDiskEncryptionType(diskParams.DiskEncryptionType); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	networkAccessPolicy, err := azureutils.NormalizeNetworkAccessPolicy(diskParams.NetworkAccessPolicy)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	publicNetworkAccess, err := azureutils.NormalizePublicNetworkAccess(diskParams.PublicNetworkAccess)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	selectedAvailabilityZone := azureutils.PickAvailabilityZone(req.GetAccessibilityRequirements(), d.cloud.Location, topologyKey)

	if d.enableDiskCapacityCheck {
		if ok, err := d.checkDiskCapacity(ctx, diskParams.SubscriptionID, diskParams.ResourceGroup, diskParams.DiskName, requestGiB); !ok {
			return nil, err
		}
	}

	klog.V(2).Infof("begin to create azure disk(%s) account type(%s) rg(%s) location(%s) size(%d) diskZone(%v) maxShares(%d)",
		diskParams.DiskName, skuName, diskParams.ResourceGroup, diskParams.Location, requestGiB, selectedAvailabilityZone, diskParams.MaxShares)

	contentSource := &csi.VolumeContentSource{}

	if strings.EqualFold(diskParams.WriteAcceleratorEnabled, consts.TrueValue) {
		diskParams.Tags[azure.WriteAcceleratorEnabled] = consts.TrueValue
	}
	sourceID := ""
	sourceType := ""
	content := req.GetVolumeContentSource()
	if content != nil {
		if content.GetSnapshot() != nil {
			sourceID = content.GetSnapshot().GetSnapshotId()
			sourceType = consts.SourceSnapshot
			contentSource = &csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Snapshot{
					Snapshot: &csi.VolumeContentSource_SnapshotSource{
						SnapshotId: sourceID,
					},
				},
			}
		} else {
			sourceID = content.GetVolume().GetVolumeId()
			sourceType = consts.SourceVolume
			contentSource = &csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Volume{
					Volume: &csi.VolumeContentSource_VolumeSource{
						VolumeId: sourceID,
					},
				},
			}

			subsID, resourceGroup, diskName, err := azureutils.GetInfoFromURI(sourceID)
			if err != nil {
				return nil, status.Errorf(codes.NotFound, "%v", err)
			}
			if sourceGiB, _, _ := d.GetSourceDiskSize(ctx, subsID, resourceGroup, diskName, 0, consts.SourceDiskSearchMaxDepth); sourceGiB != nil && *sourceGiB < int32(requestGiB) {
				diskParams.VolumeContext[consts.ResizeRequired] = strconv.FormatBool(true)
			}
		}
	}

	diskParams.VolumeContext[consts.RequestedSizeGib] = strconv.Itoa(requestGiB)
	volumeOptions := &ManagedDiskOptions{
		AvailabilityZone:    selectedAvailabilityZone,
		BurstingEnabled:     diskParams.EnableBursting,
		DiskEncryptionSetID: diskParams.DiskEncryptionSetID,
		DiskIOPSReadWrite:   diskParams.DiskIOPSReadWrite,
		DiskMBpsReadWrite:   diskParams.DiskMBPSReadWrite,
		DiskName:            diskParams.DiskName,
		LogicalSectorSize:   int32(diskParams.LogicalSectorSize),
		MaxShares:           int32(diskParams.MaxShares),
		ResourceGroup:       diskParams.ResourceGroup,
		SubscriptionID:      diskParams.SubscriptionID,
		SizeGB:              requestGiB,
		StorageAccountType:  skuName,
		SourceResourceID:    sourceID,
		SourceType:          sourceType,
		Tags:                diskParams.Tags,
		Location:            diskParams.Location,
		PerformancePlus:     diskParams.PerformancePlus,
	}
	// Azure Stack Cloud does not support NetworkAccessPolicy, PublicNetworkAccess
	if !azureutils.IsAzureStackCloud(d.cloud.Config.Cloud, d.cloud.Config.DisableAzureStackCloud) {
		volumeOptions.NetworkAccessPolicy = networkAccessPolicy
		volumeOptions.PublicNetworkAccess = publicNetworkAccess
		if diskParams.DiskAccessID != "" {
			volumeOptions.DiskAccessID = &diskParams.DiskAccessID
		}
	}

	var diskURI string
	mc := metrics.NewMetricContext(consts.AzureDiskCSIDriverName, "controller_create_volume", d.cloud.ResourceGroup, d.cloud.SubscriptionID, d.Name)
	isOperationSucceeded := false
	defer func() {
		mc.ObserveOperationWithResult(isOperationSucceeded, consts.VolumeID, diskURI)
	}()

	diskURI, err = d.diskController.CreateManagedDisk(ctx, volumeOptions)
	if err != nil {
		if strings.Contains(err.Error(), consts.NotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	isOperationSucceeded = true
	klog.V(2).Infof("create azure disk(%s) account type(%s) rg(%s) location(%s) size(%d) tags(%s) successfully", diskParams.DiskName, skuName, diskParams.ResourceGroup, diskParams.Location, requestGiB, diskParams.Tags)

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      diskURI,
			CapacityBytes: volumehelper.GiBToBytes(int64(requestGiB)),
			VolumeContext: diskParams.VolumeContext,
			ContentSource: contentSource,
			AccessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{topologyKey: selectedAvailabilityZone},
				},
			},
		},
	}, nil
}

// DeleteVolume delete an azure disk
func (d *DriverV2) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in the request")
	}

	if err := d.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid delete volume req: %v", req)
	}
	diskURI := volumeID

	if !azureutils.IsARMResourceID(diskURI) {
		klog.Errorf("diskURI(%s) is not a valid ARM resource ID", diskURI)
		return &csi.DeleteVolumeResponse{}, nil
	}

	if acquired := d.volumeLocks.TryAcquire(volumeID); !acquired {
		return nil, status.Errorf(codes.Aborted, volumeOperationAlreadyExistsFmt, volumeID)
	}
	defer d.volumeLocks.Release(volumeID)

	mc := metrics.NewMetricContext(consts.AzureDiskCSIDriverName, "controller_delete_volume", d.cloud.ResourceGroup, d.cloud.SubscriptionID, d.Name)
	isOperationSucceeded := false
	defer func() {
		mc.ObserveOperationWithResult(isOperationSucceeded, consts.VolumeID, diskURI)
	}()

	klog.V(2).Infof("deleting azure disk(%s)", diskURI)
	err := d.diskController.DeleteManagedDisk(ctx, diskURI)
	klog.V(2).Infof("delete azure disk(%s) returned with %v", diskURI, err)
	isOperationSucceeded = (err == nil)
	return &csi.DeleteVolumeResponse{}, err
}

// ControllerGetVolume get volume
func (d *DriverV2) ControllerGetVolume(context.Context, *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerModifyVolume modify volume
func (d *DriverV2) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in the request")
	}

	if err := d.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_MODIFY_VOLUME); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid modify volume req: %v", req)
	}
	diskURI := volumeID
	if _, err := d.checkDiskExists(ctx, diskURI); err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("Volume not found, failed with error: %v", err))
	}

	diskParams, err := azureutils.ParseDiskParameters(req.GetMutableParameters())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Failed parsing disk parameters: %v", err)
	}

	// normalize values
	skuName, err := azureutils.NormalizeStorageAccountType(diskParams.AccountType, d.cloud.Config.Cloud, d.cloud.Config.DisableAzureStackCloud)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if diskParams.AccountType == "" {
		skuName = ""
	}

	klog.V(2).Infof("begin to modify azure disk(%s) account type(%s) rg(%s) location(%s)",
		diskParams.DiskName, skuName, diskParams.ResourceGroup, diskParams.Location)

	volumeOptions := &ManagedDiskOptions{
		DiskIOPSReadWrite:  diskParams.DiskIOPSReadWrite,
		DiskMBpsReadWrite:  diskParams.DiskMBPSReadWrite,
		ResourceGroup:      diskParams.ResourceGroup,
		SubscriptionID:     diskParams.SubscriptionID,
		StorageAccountType: skuName,
		SourceResourceID:   diskURI,
		SourceType:         consts.SourceVolume,
	}

	mc := metrics.NewMetricContext(consts.AzureDiskCSIDriverName, "controller_modify_volume", d.cloud.ResourceGroup, d.cloud.SubscriptionID, d.Name)
	isOperationSucceeded := false
	defer func() {
		mc.ObserveOperationWithResult(isOperationSucceeded, consts.VolumeID, diskURI)
	}()

	if err = d.diskController.ModifyDisk(ctx, volumeOptions); err != nil {
		if strings.Contains(err.Error(), consts.NotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	isOperationSucceeded = true
	klog.V(2).Infof("modify azure disk(%s) account type(%s) rg(%s) location(%s) successfully", diskParams.DiskName, skuName, diskParams.ResourceGroup, diskParams.Location)

	return &csi.ControllerModifyVolumeResponse{}, err
}

// ControllerPublishVolume attach an azure disk to a required node
func (d *DriverV2) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	diskURI := req.GetVolumeId()
	if len(diskURI) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}

	caps := []*csi.VolumeCapability{volCap}
	maxShares, err := azureutils.GetMaxShares(req.GetVolumeContext())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "MaxShares value not supported")
	}

	if err := azureutils.IsValidVolumeCapabilities(caps, maxShares); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	disk, err := d.checkDiskExists(ctx, diskURI)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("Volume not found, failed with error: %v", err))
	}

	nodeID := req.GetNodeId()
	if len(nodeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Node ID not provided")
	}

	nodeName := types.NodeName(nodeID)
	_, _, diskName, err := azureutils.GetInfoFromURI(diskURI)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	mc := metrics.NewMetricContext(consts.AzureDiskCSIDriverName, "controller_publish_volume", d.cloud.ResourceGroup, d.cloud.SubscriptionID, d.Name)
	isOperationSucceeded := false
	defer func() {
		mc.ObserveOperationWithResult(isOperationSucceeded, consts.VolumeID, diskURI, consts.Node, string(nodeName))
	}()

	lun, vmState, err := d.diskController.GetDiskLun(ctx, diskName, diskURI, nodeName)
	if err == cloudprovider.InstanceNotFound {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("failed to get azure instance id for node %q (%v)", nodeName, err))
	}

	klog.V(2).Infof("GetDiskLun returned: %v. Initiating attaching volume %s to node %s.", err, diskURI, nodeName)

	volumeContext := req.GetVolumeContext()
	if volumeContext == nil {
		volumeContext = map[string]string{}
	}

	if err == nil {
		if vmState != nil && strings.ToLower(*vmState) == "failed" {
			klog.Warningf("VM(%s) is in failed state, update VM first", nodeName)
			if err := d.diskController.UpdateVM(ctx, nodeName); err != nil {
				return nil, status.Errorf(codes.Internal, "update instance %q failed with %v", nodeName, err)
			}
		}
		// Volume is already attached to node.
		klog.V(2).Infof("Attach operation is successful. volume %s is already attached to node %s at lun %d.", diskURI, nodeName, lun)
	} else {
		if azureutils.IsThrottlingError(err) {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
		var cachingMode armcompute.CachingTypes
		if cachingMode, err = azureutils.GetCachingMode(volumeContext); err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
		klog.V(2).Infof("Trying to attach volume %s to node %s", diskURI, nodeName)

		lun, err = d.diskController.AttachDisk(ctx, diskName, diskURI, nodeName, cachingMode, disk, nil)
		if err == nil {
			klog.V(2).Infof("Attach operation successful: volume %s attached to node %s.", diskURI, nodeName)
		} else {
			if derr, ok := err.(*volerr.DanglingAttachError); ok {
				klog.Warningf("volume %s is already attached to node %s, try detach first", diskURI, derr.CurrentNode)
				if err = d.diskController.DetachDisk(ctx, diskName, diskURI, derr.CurrentNode); err != nil {
					return nil, status.Errorf(codes.Internal, "Could not detach volume %s from node %s: %v", diskURI, derr.CurrentNode, err)
				}
				klog.V(2).Infof("Trying to attach volume %s to node %s again", diskURI, nodeName)
				lun, err = d.diskController.AttachDisk(ctx, diskName, diskURI, nodeName, cachingMode, disk, nil)
			}
			if err != nil {
				klog.Errorf("Attach volume %s to instance %s failed with %v", diskURI, nodeName, err)
				return nil, status.Errorf(codes.Internal, "Attach volume %s to instance %s failed with %v", diskURI, nodeName, err)
			}
		}
		klog.V(2).Infof("attach volume %s to node %s successfully", diskURI, nodeName)
	}

	publishContext := map[string]string{consts.LUN: strconv.Itoa(int(lun))}
	if disk != nil {
		if _, ok := volumeContext[consts.RequestedSizeGib]; !ok {
			klog.V(2).Infof("found static PV(%s), insert disk properties to volumeattachments", diskURI)
			azureutils.InsertDiskProperties(disk, publishContext)
		}
	}
	isOperationSucceeded = true
	return &csi.ControllerPublishVolumeResponse{PublishContext: publishContext}, nil
}

// ControllerUnpublishVolume detach an azure disk from a required node
func (d *DriverV2) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	diskURI := req.GetVolumeId()
	if len(diskURI) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	nodeID := req.GetNodeId()
	if len(nodeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Node ID not provided")
	}
	nodeName := types.NodeName(nodeID)

	_, _, diskName, err := azureutils.GetInfoFromURI(diskURI)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	mc := metrics.NewMetricContext(consts.AzureDiskCSIDriverName, "controller_unpublish_volume", d.cloud.ResourceGroup, d.cloud.SubscriptionID, d.Name)
	isOperationSucceeded := false
	defer func() {
		mc.ObserveOperationWithResult(isOperationSucceeded, consts.VolumeID, diskURI, consts.Node, string(nodeName))
	}()

	klog.V(2).Infof("Trying to detach volume %s from node %s", diskURI, nodeID)

	if err := d.diskController.DetachDisk(ctx, diskName, diskURI, nodeName); err != nil {
		if strings.Contains(err.Error(), consts.ErrDiskNotFound) {
			klog.Warningf("volume %s already detached from node %s", diskURI, nodeID)
		} else {
			return nil, status.Errorf(codes.Internal, "Could not detach volume %q from node %q: %v", diskURI, nodeID, err)
		}
	}
	klog.V(2).Infof("detach volume %s from node %s successfully", diskURI, nodeID)
	isOperationSucceeded = true

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// ValidateVolumeCapabilities return the capabilities of the volume
func (d *DriverV2) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	diskURI := req.GetVolumeId()
	if len(diskURI) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in the request")
	}
	volumeCapabilities := req.GetVolumeCapabilities()
	if volumeCapabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeCapabilities missing in the request")
	}

	params := req.GetParameters()
	maxShares, err := azureutils.GetMaxShares(params)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "MaxShares value not supported")
	}

	if err := azureutils.IsValidVolumeCapabilities(volumeCapabilities, maxShares); err != nil {
		return &csi.ValidateVolumeCapabilitiesResponse{Message: err.Error()}, nil
	}

	if _, err := d.checkDiskExists(ctx, diskURI); err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("Volume not found, failed with error: %v", err))
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: volumeCapabilities,
		}}, nil
}

// ControllerGetCapabilities returns the capabilities of the Controller plugin
func (d *DriverV2) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: d.Cap,
	}, nil
}

// GetCapacity returns the capacity of the total available storage pool
func (d *DriverV2) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ListVolumes return all available volumes
func (d *DriverV2) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	start := 0
	if req.StartingToken != "" {
		var err error
		start, err = strconv.Atoi(req.StartingToken)
		if err != nil {
			return nil, status.Errorf(codes.Aborted, "ListVolumes starting token(%s) parsing with error: %v", req.StartingToken, err)
		}
		if start < 0 {
			return nil, status.Errorf(codes.Aborted, "ListVolumes starting token(%d) can not be negative", start)
		}
	}
	if d.cloud.KubeClient != nil && d.cloud.KubeClient.CoreV1() != nil && d.cloud.KubeClient.CoreV1().PersistentVolumes() != nil {
		klog.V(6).Infof("List Volumes in Cluster:")
		return d.listVolumesInCluster(ctx, start, int(req.MaxEntries))
	}
	klog.V(6).Infof("List Volumes in Node Resource Group: %s", d.cloud.ResourceGroup)
	return d.listVolumesInNodeResourceGroup(ctx, start, int(req.MaxEntries))
}

// listVolumesInCluster is a helper function for ListVolumes used for when there is an available kubeclient
func (d *DriverV2) listVolumesInCluster(ctx context.Context, start, maxEntries int) (*csi.ListVolumesResponse, error) {
	pvList, err := d.cloud.KubeClient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ListVolumes failed while fetching PersistentVolumes List with error: %v", err)
	}

	// get all resource groups and put them into a sorted slice
	rgMap := make(map[string]bool)
	volSet := make(map[string]bool)
	for _, pv := range pvList.Items {
		if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == d.Name {
			diskURI := pv.Spec.CSI.VolumeHandle
			_, rg, _, err := azureutils.GetInfoFromURI(diskURI)
			if err != nil {
				klog.Warningf("failed to get subscription id, resource group from disk uri (%s) with error(%v)", diskURI, err)
				continue
			}
			rg, diskURI = strings.ToLower(rg), strings.ToLower(diskURI)
			volSet[diskURI] = true
			if _, visited := rgMap[rg]; visited {
				continue
			}
			rgMap[rg] = true
		}
	}

	resourceGroups := make([]string, len(rgMap))
	i := 0
	for rg := range rgMap {
		resourceGroups[i] = rg
		i++
	}
	sort.Strings(resourceGroups)

	// loop through each resourceGroup to get disk lists
	entries := []*csi.ListVolumesResponse_Entry{}
	numVisited := 0
	isCompleteRun, startFound := true, false
	for _, resourceGroup := range resourceGroups {
		if !isCompleteRun || (maxEntries > 0 && len(entries) >= maxEntries) {
			isCompleteRun = false
			break
		}
		localStart := start - numVisited
		if startFound {
			localStart = 0
		}
		listStatus := d.listVolumesByResourceGroup(ctx, resourceGroup, entries, localStart, maxEntries-len(entries), volSet)
		numVisited += listStatus.numVisited
		if listStatus.err != nil {
			if status.Code(listStatus.err) == codes.FailedPrecondition {
				continue
			}
			return nil, listStatus.err
		}
		startFound = true
		entries = listStatus.entries
		isCompleteRun = isCompleteRun && listStatus.isCompleteRun
	}
	// if start was not found, start token was greater than total number of disks
	if start > 0 && !startFound {
		return nil, status.Errorf(codes.FailedPrecondition, "ListVolumes starting token(%d) is greater than total number of disks", start)
	}

	nextTokenString := ""
	if !isCompleteRun {
		nextTokenString = strconv.Itoa(start + numVisited)
	}

	listVolumesResp := &csi.ListVolumesResponse{
		Entries:   entries,
		NextToken: nextTokenString,
	}

	return listVolumesResp, nil
}

// listVolumesInNodeResourceGroup is a helper function for ListVolumes used for when there is no available kubeclient
func (d *DriverV2) listVolumesInNodeResourceGroup(ctx context.Context, start, maxEntries int) (*csi.ListVolumesResponse, error) {
	entries := []*csi.ListVolumesResponse_Entry{}
	listStatus := d.listVolumesByResourceGroup(ctx, d.cloud.ResourceGroup, entries, start, maxEntries, nil)
	if listStatus.err != nil {
		return nil, listStatus.err
	}

	nextTokenString := ""
	if !listStatus.isCompleteRun {
		nextTokenString = strconv.Itoa(listStatus.numVisited)
	}

	listVolumesResp := &csi.ListVolumesResponse{
		Entries:   listStatus.entries,
		NextToken: nextTokenString,
	}

	return listVolumesResp, nil
}

// listVolumesByResourceGroup is a helper function that updates the ListVolumeResponse_Entry slice and returns number of total visited volumes, number of volumes that needs to be visited and an error if found
func (d *DriverV2) listVolumesByResourceGroup(ctx context.Context, resourceGroup string, entries []*csi.ListVolumesResponse_Entry, start, maxEntries int, volSet map[string]bool) listVolumeStatus {
	diskClient := d.clientFactory.GetDiskClient()
	disks, derr := diskClient.List(ctx, resourceGroup)
	if derr != nil {
		return listVolumeStatus{err: status.Errorf(codes.Internal, "ListVolumes on rg(%s) failed with error: %v", resourceGroup, derr)}
	}
	// if volSet is initialized but is empty, return
	if volSet != nil && len(volSet) == 0 {
		return listVolumeStatus{
			numVisited:    len(disks),
			isCompleteRun: true,
			entries:       entries,
		}
	}
	if start > 0 && start >= len(disks) {
		return listVolumeStatus{
			numVisited: len(disks),
			err:        status.Errorf(codes.FailedPrecondition, "ListVolumes starting token(%d) on rg(%s) is greater than total number of volumes", start, d.cloud.ResourceGroup),
		}
	}
	if start < 0 {
		start = 0
	}
	i := start
	isCompleteRun := true
	// Loop until
	for ; i < len(disks); i++ {
		if maxEntries > 0 && len(entries) >= maxEntries {
			isCompleteRun = false
			break
		}

		disk := disks[i]
		// if given a set of volumes from KubeClient, only continue if the disk can be found in the set
		if volSet != nil && !volSet[strings.ToLower(*disk.ID)] {
			continue
		}
		// HyperVGeneration property is only setup for os disks. Only the non os disks should be included in the list
		if disk.Properties == nil || disk.Properties.HyperVGeneration == nil || *disk.Properties.HyperVGeneration == "" {
			nodeList := []string{}

			if disk.ManagedBy != nil {
				attachedNode, err := d.cloud.VMSet.GetNodeNameByProviderID(ctx, *disk.ManagedBy)
				if err != nil {
					return listVolumeStatus{err: err}
				}
				nodeList = append(nodeList, string(attachedNode))
			}

			entries = append(entries, &csi.ListVolumesResponse_Entry{
				Volume: &csi.Volume{
					VolumeId: *disk.ID,
				},
				Status: &csi.ListVolumesResponse_VolumeStatus{
					PublishedNodeIds: nodeList,
				},
			})
		}
	}
	return listVolumeStatus{
		numVisited:    i - start,
		isCompleteRun: isCompleteRun,
		entries:       entries,
	}
}

// ControllerExpandVolume controller expand volume
func (d *DriverV2) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in the request")
	}
	if err := d.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_EXPAND_VOLUME); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid expand volume request: %v", req)
	}

	capacityBytes := req.GetCapacityRange().GetRequiredBytes()
	if capacityBytes == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capacity range missing in request")
	}
	requestSize := *resource.NewQuantity(capacityBytes, resource.BinarySI)

	diskURI := req.GetVolumeId()
	subsID, resourceGroup, diskName, err := azureutils.GetInfoFromURI(diskURI)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	mc := metrics.NewMetricContext(consts.AzureDiskCSIDriverName, "controller_expand_volume", d.cloud.ResourceGroup, d.cloud.SubscriptionID, d.Name)
	isOperationSucceeded := false
	defer func() {
		mc.ObserveOperationWithResult(isOperationSucceeded, consts.VolumeID, diskURI)
	}()

	diskClient, err := d.clientFactory.GetDiskClientForSub(subsID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get disk client for subscription(%s) with error(%v)", subsID, err)
	}
	result, err := diskClient.Get(ctx, resourceGroup, diskName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get the disk(%s) under rg(%s) with error(%v)", diskName, resourceGroup, err)
	}
	if result.Properties.DiskSizeGB == nil {
		return nil, status.Errorf(codes.Internal, "could not get size of the disk(%s)", diskName)
	}
	oldSize := *resource.NewQuantity(int64(*result.Properties.DiskSizeGB), resource.BinarySI)

	klog.V(2).Infof("begin to expand azure disk(%s) with new size(%d)", diskURI, requestSize.Value())
	newSize, err := d.diskController.ResizeDisk(ctx, diskURI, oldSize, requestSize, d.enableDiskOnlineResize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resize disk(%s) with error(%v)", diskURI, err)
	}

	currentSize, ok := newSize.AsInt64()
	if !ok {
		return nil, status.Errorf(codes.Internal, "failed to transform disk size with error(%v)", err)
	}

	isOperationSucceeded = true
	klog.V(2).Infof("expand azure disk(%s) successfully, currentSize(%d)", diskURI, currentSize)

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         currentSize,
		NodeExpansionRequired: true,
	}, nil
}

// CreateSnapshot create a snapshot
func (d *DriverV2) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	sourceVolumeID := req.GetSourceVolumeId()
	if len(sourceVolumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshot Source Volume ID must be provided")
	}
	snapshotName := req.Name
	if len(snapshotName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "snapshot name must be provided")
	}

	snapshotName = azureutils.CreateValidDiskName(snapshotName)

	var customTags string
	// set incremental snapshot as true by default
	incremental := true
	var resourceGroup, subsID, dataAccessAuthMode, tagValueDelimiter string
	var err error

	parameters := req.GetParameters()
	for k, v := range parameters {
		switch strings.ToLower(k) {
		case consts.TagsField:
			customTags = v
		case consts.IncrementalField:
			if v == "false" {
				incremental = false
			}
		case consts.ResourceGroupField:
			resourceGroup = v
		case consts.SubscriptionIDField:
			subsID = v
		case consts.DataAccessAuthModeField:
			dataAccessAuthMode = v
		case consts.TagValueDelimiterField:
			tagValueDelimiter = v
		default:
			return nil, status.Errorf(codes.Internal, "AzureDisk - invalid option %s in VolumeSnapshotClass", k)
		}
	}

	if azureutils.IsAzureStackCloud(d.cloud.Config.Cloud, d.cloud.Config.DisableAzureStackCloud) {
		klog.V(2).Info("Use full snapshot instead as Azure Stack does not support incremental snapshot.")
		incremental = false
	}

	if resourceGroup == "" {
		if _, resourceGroup, _, err = azureutils.GetInfoFromURI(sourceVolumeID); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "could not get resource group from diskURI(%s) with error(%v)", sourceVolumeID, err)
		}
	}

	customTagsMap, err := volumehelper.ConvertTagsToMap(customTags, tagValueDelimiter)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	tags := make(map[string]*string)
	for k, v := range customTagsMap {
		value := v
		tags[k] = &value
	}

	snapshot := armcompute.Snapshot{
		Properties: &armcompute.SnapshotProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: &sourceVolumeID,
			},
			Incremental: &incremental,
		},
		Location: &d.cloud.Location,
		Tags:     tags,
	}
	if dataAccessAuthMode != "" {
		if err := azureutils.ValidateDataAccessAuthMode(dataAccessAuthMode); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		snapshot.Properties.DataAccessAuthMode = to.Ptr(armcompute.DataAccessAuthMode(dataAccessAuthMode))
	}

	mc := metrics.NewMetricContext(consts.AzureDiskCSIDriverName, "controller_create_snapshot", d.cloud.ResourceGroup, d.cloud.SubscriptionID, d.Name)
	isOperationSucceeded := false
	defer func() {
		mc.ObserveOperationWithResult(isOperationSucceeded, consts.SourceResourceID, sourceVolumeID, consts.SnapshotName, snapshotName)
	}()

	klog.V(2).Infof("begin to create snapshot(%s, incremental: %v) under rg(%s)", snapshotName, incremental, resourceGroup)
	snapshotClient, err := d.clientFactory.GetSnapshotClientForSub(subsID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get snapshot client for subscription(%s) with error(%v)", subsID, err)
	}
	if _, err := snapshotClient.CreateOrUpdate(ctx, resourceGroup, snapshotName, snapshot); err != nil {
		if strings.Contains(err.Error(), "existing disk") {
			return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("request snapshot(%s) under rg(%s) already exists, but the SourceVolumeId is different, error details: %v", snapshotName, resourceGroup, err))
		}
		azureutils.SleepIfThrottled(err, consts.SnapshotOpThrottlingSleepSec)
		return nil, status.Error(codes.Internal, fmt.Sprintf("create snapshot error: %v", err))
	}
	if err := d.waitForSnapshotReady(ctx, subsID, resourceGroup, snapshotName, waitForSnapshotReadyInterval, waitForSnapshotReadyTimeout); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("waitForSnapshotReady(%s, %s, %s) failed with %v", subsID, resourceGroup, snapshotName, err))
	}
	klog.V(2).Infof("create snapshot(%s) under rg(%s) successfully", snapshotName, resourceGroup)

	csiSnapshot, err := d.getSnapshotByID(ctx, subsID, resourceGroup, snapshotName, sourceVolumeID)
	if err != nil {
		return nil, err
	}

	createResp := &csi.CreateSnapshotResponse{
		Snapshot: csiSnapshot,
	}
	isOperationSucceeded = true
	return createResp, nil
}

// DeleteSnapshot delete a snapshot
func (d *DriverV2) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	snapshotID := req.SnapshotId
	if len(snapshotID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Snapshot ID must be provided")
	}

	var err error
	var subsID string
	snapshotName := snapshotID
	resourceGroup := d.cloud.ResourceGroup

	if azureutils.IsARMResourceID(snapshotID) {
		subsID, resourceGroup, snapshotName, err = azureutils.GetInfoFromURI(snapshotID)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
	}
	mc := metrics.NewMetricContext(consts.AzureDiskCSIDriverName, "controller_delete_snapshot", d.cloud.ResourceGroup, d.cloud.SubscriptionID, d.Name)
	isOperationSucceeded := false
	defer func() {
		mc.ObserveOperationWithResult(isOperationSucceeded, consts.SnapshotID, snapshotName)
	}()

	klog.V(2).Infof("begin to delete snapshot(%s) under rg(%s)", snapshotName, resourceGroup)
	snapshotClient, err := d.clientFactory.GetSnapshotClientForSub(subsID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get snapshot client for subscription(%s) with error(%v)", subsID, err)
	}
	err = snapshotClient.Delete(ctx, resourceGroup, snapshotName)
	if err != nil {
		azureutils.SleepIfThrottled(err, consts.SnapshotOpThrottlingSleepSec)
		return nil, status.Error(codes.Internal, fmt.Sprintf("delete snapshot error: %v", err))
	}
	klog.V(2).Infof("delete snapshot(%s) under rg(%s) successfully", snapshotName, resourceGroup)
	isOperationSucceeded = true
	return &csi.DeleteSnapshotResponse{}, nil
}

// ListSnapshots list all snapshots
func (d *DriverV2) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	// SnapshotId is not empty, return snapshot that match the snapshot id.
	if len(req.GetSnapshotId()) != 0 {
		snapshot, err := d.getSnapshotByID(ctx, "", d.cloud.ResourceGroup, req.GetSnapshotId(), req.SourceVolumeId)
		if err != nil {
			if strings.Contains(err.Error(), consts.ResourceNotFound) {
				return &csi.ListSnapshotsResponse{}, nil
			}
			return nil, err
		}
		entries := []*csi.ListSnapshotsResponse_Entry{
			{
				Snapshot: snapshot,
			},
		}
		listSnapshotResp := &csi.ListSnapshotsResponse{
			Entries: entries,
		}
		return listSnapshotResp, nil
	}
	snapshotClient := d.clientFactory.GetSnapshotClient()
	// no SnapshotId is set, return all snapshots that satisfy the request.
	snapshots, err := snapshotClient.List(ctx, d.cloud.ResourceGroup)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Unknown list snapshot error: %v", err))
	}

	return azureutils.GetEntriesAndNextToken(req, snapshots)
}

func (d *DriverV2) getSnapshotByID(ctx context.Context, subsID, resourceGroup, snapshotID, sourceVolumeID string) (*csi.Snapshot, error) {
	var err error
	snapshotName := snapshotID
	if azureutils.IsARMResourceID(snapshotID) {
		subsID, resourceGroup, snapshotName, err = azureutils.GetInfoFromURI(snapshotID)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
	}
	snapshotClient, err := d.clientFactory.GetSnapshotClientForSub(subsID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get snapshot client for subscription(%s) with error(%v)", subsID, err)
	}
	snapshot, rerr := snapshotClient.Get(ctx, resourceGroup, snapshotName)
	if rerr != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("get snapshot %s from rg(%s) error: %v", snapshotName, resourceGroup, rerr))
	}

	return azureutils.GenerateCSISnapshot(sourceVolumeID, snapshot)
}

// GetSourceDiskSize recursively searches for the sourceDisk and returns: sourceDisk disk size, error
func (d *DriverV2) GetSourceDiskSize(ctx context.Context, subsID, resourceGroup, diskName string, curDepth, maxDepth int) (*int32, *armcompute.Disk, error) {
	if curDepth > maxDepth {
		return nil, nil, status.Error(codes.Internal, fmt.Sprintf("current depth (%d) surpassed the max depth (%d) while searching for the source disk size", curDepth, maxDepth))
	}
	diskClient, err := d.clientFactory.GetDiskClientForSub(subsID)
	if err != nil {
		return nil, nil, status.Error(codes.Internal, err.Error())
	}
	result, err := diskClient.Get(ctx, resourceGroup, diskName)
	if err != nil {
		return nil, result, err
	}
	if result.Properties == nil {
		return nil, result, status.Error(codes.Internal, fmt.Sprintf("DiskProperty not found for disk (%s) in resource group (%s)", diskName, resourceGroup))
	}

	if result.Properties.CreationData != nil && result.Properties.CreationData.CreateOption != nil && *result.Properties.CreationData.CreateOption == armcompute.DiskCreateOptionCopy {
		klog.V(2).Infof("Clone source disk has a parent source")
		sourceResourceID := *result.Properties.CreationData.SourceResourceID
		subsID, parentResourceGroup, parentDiskName, err := azureutils.GetInfoFromURI(sourceResourceID)
		if err != nil {
			return nil, result, status.Error(codes.Internal, fmt.Sprintf("failed to get subscription id, resource group from disk uri (%s) with error(%v)", sourceResourceID, err))
		}
		return d.GetSourceDiskSize(ctx, subsID, parentResourceGroup, parentDiskName, curDepth+1, maxDepth)
	}

	if (*result.Properties).DiskSizeGB == nil {
		return nil, result, status.Error(codes.Internal, fmt.Sprintf("DiskSizeGB for disk (%s) in resourcegroup (%s) is nil", diskName, resourceGroup))
	}
	return (*result.Properties).DiskSizeGB, result, nil
}
