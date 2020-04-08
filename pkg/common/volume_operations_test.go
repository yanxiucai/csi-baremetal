package common

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
)

func TestVolumeOperationsImpl_CreateVolume_VolumeExists(t *testing.T) {
	// 1. Volume CR has already exist
	svc := setupVOOperationsTest(t)

	v := testVolume1
	v.Spec.Status = api.OperationalStatus_Created
	err := svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
	assert.Nil(t, err)

	createdVolume1, err := svc.CreateVolume(testCtx, api.Volume{Id: v.Spec.Id})
	assert.Nil(t, err)
	assert.Equal(t, &v.Spec, createdVolume1)
}

// Volume CR was successfully created, HDD SC
func TestVolumeOperationsImpl_CreateVolume_HDDVolumeCreated(t *testing.T) {
	var (
		svc           = setupVOOperationsTest(t)
		acProvider    = &mocks.ACOperationsMock{}
		volumeID      = "pvc-aaaa-bbbb"
		ctxWithID     = context.WithValue(testCtx, base.RequestUUID, volumeID)
		requiredNode  = ""
		requiredSC    = api.StorageClass_HDD
		requiredBytes = int64(base.GBYTE)
		expectedAC    = &accrd.AvailableCapacity{
			Spec: api.AvailableCapacity{
				Location:     testDrive1UUID,
				NodeId:       testNode1Name,
				StorageClass: requiredSC,
				Size:         int64(base.GBYTE) * 42,
			},
		}
		expectedVolume = &api.Volume{
			Id:           volumeID,
			Location:     expectedAC.Spec.Location,
			StorageClass: expectedAC.Spec.StorageClass,
			NodeId:       expectedAC.Spec.NodeId,
			Size:         expectedAC.Spec.Size,
			Status:       api.OperationalStatus_Creating,
		}
	)

	svc.acProvider = acProvider
	acProvider.On("SearchAC", ctxWithID, requiredNode, requiredBytes, requiredSC).
		Return(expectedAC).Times(1)

	createdVolume, err := svc.CreateVolume(testCtx, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       requiredNode,
		Size:         requiredBytes,
	})
	assert.Nil(t, err)
	assert.Equal(t, expectedVolume, createdVolume)
}

// Volume CR was successfully created, HDDLVG SC
func TestVolumeOperationsImpl_CreateVolume_HDDLVGVolumeCreated(t *testing.T) {
	var (
		svc           = setupVOOperationsTest(t)
		acProvider    = &mocks.ACOperationsMock{}
		volumeID      = "pvc-aaaa-bbbb"
		ctxWithID     = context.WithValue(testCtx, base.RequestUUID, volumeID)
		requiredNode  = ""
		requiredSC    = api.StorageClass_HDD
		requiredBytes = int64(base.GBYTE)
		expectedAC    = &accrd.AvailableCapacity{
			Spec: api.AvailableCapacity{
				Location:     testDrive1UUID,
				NodeId:       testNode1Name,
				StorageClass: api.StorageClass_HDDLVG,
				Size:         int64(base.GBYTE) * 42,
			},
		}
	)
	svc.acProvider = acProvider
	acProvider.On("SearchAC", ctxWithID, requiredNode, requiredBytes, requiredSC).
		Return(expectedAC).Times(1)

	createdVolume, err := svc.CreateVolume(testCtx, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       requiredNode,
		Size:         requiredBytes,
	})
	assert.Nil(t, err)
	expectedVolume := &api.Volume{
		Id:           volumeID,
		Location:     expectedAC.Spec.Location,
		StorageClass: expectedAC.Spec.StorageClass,
		NodeId:       expectedAC.Spec.NodeId,
		Size:         requiredBytes,
		Status:       api.OperationalStatus_Creating,
	}
	assert.Equal(t, expectedVolume, createdVolume)
}

// Volume CR exists and timeout for creation exceeded
func TestVolumeOperationsImpl_CreateVolume_FailCauseTimeout(t *testing.T) {
	var (
		svc = setupVOOperationsTest(t)
		v   = testVolume1
	)
	v.ObjectMeta.CreationTimestamp = v1.Time{
		Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local),
	}
	err := svc.k8sClient.CreateCR(testCtx, v.Name, &v)
	assert.Nil(t, err)

	createdVolume, err := svc.CreateVolume(testCtx, api.Volume{Id: v.Name})
	assert.NotNil(t, err)
	assert.Equal(t, status.Error(codes.Internal, "Unable to create volume in allocated time"), err)
	assert.Nil(t, createdVolume)
}

// There is no suitable AC
func TestVolumeOperationsImpl_CreateVolume_FailNoAC(t *testing.T) {
	var (
		svc           = setupVOOperationsTest(t)
		acProvider    = &mocks.ACOperationsMock{}
		volumeID      = "pvc-aaaa-bbbb"
		ctxWithID     = context.WithValue(testCtx, base.RequestUUID, volumeID)
		requiredNode  = ""
		requiredSC    = api.StorageClass_HDD
		requiredBytes = int64(base.GBYTE)
	)

	svc.acProvider = acProvider
	acProvider.On("SearchAC", ctxWithID, requiredNode, requiredBytes, requiredSC).
		Return(nil).Times(1)

	createdVolume, err := svc.CreateVolume(testCtx, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       requiredNode,
		Size:         requiredBytes,
	})
	assert.NotNil(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Nil(t, createdVolume)
}

func TestVolumeOperationsImpl_DeleteVolume_NotFound(t *testing.T) {
	svc := setupVOOperationsTest(t)

	err := svc.DeleteVolume(testCtx, "unknown-volume")
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))
}

func TestVolumeOperationsImpl_DeleteVolume_FailToRemoveSt(t *testing.T) {
	var (
		svc = setupVOOperationsTest(t)
		v   = testVolume1
		err error
	)

	v.Spec.Status = api.OperationalStatus_FailToRemove
	err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
	assert.Nil(t, err)

	err = svc.DeleteVolume(testCtx, testVolume1Name)
	assert.NotNil(t, err)
	assert.Equal(t, status.Error(codes.Internal, "volume has reached FailToRemove status"), err)
}

// volume has status Removed or Removing
func TestVolumeOperationsImpl_DeleteVolume(t *testing.T) {
	var (
		svc = setupVOOperationsTest(t)
		v   = testVolume1
		err error
	)

	for _, st := range []api.OperationalStatus{api.OperationalStatus_Removing, api.OperationalStatus_Removed} {
		v.Spec.Status = st
		err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
		assert.Nil(t, err)

		err = svc.DeleteVolume(testCtx, testVolume1Name)
		assert.Nil(t, err)
	}
}

func TestVolumeOperationsImpl_DeleteVolume_SetStatus(t *testing.T) {
	var (
		svc        = setupVOOperationsTest(t)
		v          = testVolume1
		updatedVol = volumecrd.Volume{}
		err        error
	)

	v.Spec.Status = api.OperationalStatus_ReadyToRemove
	err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
	assert.Nil(t, err)

	err = svc.DeleteVolume(testCtx, testVolume1Name)
	assert.Nil(t, err)

	err = svc.k8sClient.ReadCR(testCtx, testVolume1Name, &updatedVol)
	assert.Nil(t, err)
	assert.Equal(t, api.OperationalStatus_Removing, updatedVol.Spec.Status)
}

func TestVolumeOperationsImpl_WaitStatus_Success(t *testing.T) {
	svc := setupVOOperationsTest(t)

	v := testVolume1
	v.Spec.Status = api.OperationalStatus_Created
	err := svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
	assert.Nil(t, err)

	ctx, closeFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeFn()

	reached, st := svc.WaitStatus(ctx, v.Name, api.OperationalStatus_FailedToCreate, api.OperationalStatus_Created)
	assert.True(t, reached)
	assert.Equal(t, api.OperationalStatus_Created, st)
}

func TestVolumeOperationsImpl_WaitStatus_Fails(t *testing.T) {
	svc := setupVOOperationsTest(t)

	var (
		reached bool
		status  api.OperationalStatus
	)

	// volume CR wasn't found scenario
	reached, status = svc.WaitStatus(testCtx, "unknown_name", api.OperationalStatus_Created)
	assert.False(t, reached)
	assert.Equal(t, api.OperationalStatus(-1), status)

	// ctx is done scenario
	err := svc.k8sClient.CreateCR(testCtx, testVolume1Name, &testVolume1)
	assert.Nil(t, err)

	ctx, closeFn := context.WithTimeout(context.Background(), 10*time.Second)
	closeFn()
	ctx.Done()

	// volume CR wasn't found
	reached, status = svc.WaitStatus(ctx, testVolume1Name, api.OperationalStatus_Created)
	assert.False(t, reached)
	assert.Equal(t, api.OperationalStatus(-1), status)
}

func TestVolumeOperationsImpl_UpdateCRsAfterVolumeDeletion(t *testing.T) {
	var err error

	svc1 := setupVOOperationsTest(t)

	// 1. volume with HDDLVG SC, corresponding AC should be increased, volume CR should be removed
	v1 := testVolume1
	err = svc1.k8sClient.CreateCR(testCtx, testVolume1Name, &v1)
	assert.Nil(t, err)

	svc1.UpdateCRsAfterVolumeDeletion(testCtx, testVolume1Name)

	err = svc1.k8sClient.ReadCR(testCtx, testVolume1Name, &volumecrd.Volume{})
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	// create AC, LVG and Volume
	err = svc1.k8sClient.CreateCR(testCtx, testAC4Name, &testAC4)
	assert.Nil(t, err)
	err = svc1.k8sClient.CreateCR(testCtx, testLVGName, &testLVG)
	assert.Nil(t, err)
	v1.Spec.StorageClass = api.StorageClass_HDDLVG
	v1.Spec.Location = testLVGName
	err = svc1.k8sClient.CreateCR(testCtx, testVolume1Name, &v1)
	assert.Nil(t, err)

	svc1.UpdateCRsAfterVolumeDeletion(testCtx, testVolume1Name)
	// check that Volume was removed
	err = svc1.k8sClient.ReadCR(testCtx, testVolume1Name, &volumecrd.Volume{})
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	// check that AC size was increased
	var updatedAC = &accrd.AvailableCapacity{}
	err = svc1.k8sClient.ReadCR(testCtx, testAC4Name, updatedAC)
	assert.Nil(t, err)
	assert.Equal(t, testAC4.Spec.Size+v1.Spec.Size, updatedAC.Spec.Size)

	// 2. volume with HDDLVG SC, corresponding AC is not exist and should be created, volume CR should be removed
	svc2 := setupVOOperationsTest(t)
	// Create Volume and LVG
	v2 := testVolume1
	v2.Spec.StorageClass = api.StorageClass_HDDLVG
	err = svc2.k8sClient.CreateCR(testCtx, testVolume1Name, &v2)
	assert.Nil(t, err)
	err = svc2.k8sClient.CreateCR(testCtx, testLVGName, &testLVG)

	svc2.UpdateCRsAfterVolumeDeletion(testCtx, testVolume1Name)
	// check that Volume was removed
	err = svc2.k8sClient.ReadCR(testCtx, testVolume1Name, &volumecrd.Volume{})
	assert.True(t, k8sError.IsNotFound(err))
	// check that AC CR was created
	var acList = &accrd.AvailableCapacityList{}
	err = svc2.k8sClient.ReadList(testCtx, acList)
	assert.Equal(t, 1, len(acList.Items))
	ac := acList.Items[0]
	assert.Equal(t, v2.Spec.Location, ac.Spec.Location)
	assert.Equal(t, v2.Spec.Size, ac.Spec.Size)
	assert.Equal(t, v2.Spec.NodeId, ac.Spec.NodeId)
	assert.Equal(t, v2.Spec.StorageClass, ac.Spec.StorageClass)
}

func TestVolumeOperationsImpl_ReadVolumeAndChangeStatus(t *testing.T) {
	svc := setupVOOperationsTest(t)

	var (
		v             = testVolume1
		updatedVolume = volumecrd.Volume{}
		newStatus     = api.OperationalStatus_Created
		err           error
	)

	v.Spec.Status = api.OperationalStatus_Creating
	err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
	assert.Nil(t, err)

	err = svc.ReadVolumeAndChangeStatus(testVolume1Name, newStatus)
	assert.Nil(t, err)

	err = svc.k8sClient.ReadCR(testCtx, testVolume1Name, &updatedVolume)
	assert.Nil(t, err)
	assert.Equal(t, newStatus, updatedVolume.Spec.Status)

	// volume doesn't exist scenario
	err = svc.ReadVolumeAndChangeStatus("notExisting", newStatus)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// creates fake k8s client and creates AC CRs based on provided acs
// returns instance of ACOperationsImpl based on created k8s client
func setupVOOperationsTest(t *testing.T) *VolumeOperationsImpl {
	k8sClient, err := base.GetFakeKubeClient(testNS)
	assert.Nil(t, err)
	assert.NotNil(t, k8sClient)

	return NewVolumeOperationsImpl(k8sClient, logrus.New())
}
