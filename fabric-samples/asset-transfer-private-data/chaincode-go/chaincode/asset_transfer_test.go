/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package chaincode_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/hyperledger/fabric-chaincode-go/v2/pkg/cid"
	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"

	"github.com/hyperledger/fabric-samples/asset-transfer-private-data/chaincode-go/chaincode"
	"github.com/hyperledger/fabric-samples/asset-transfer-private-data/chaincode-go/chaincode/mocks"
	"github.com/stretchr/testify/require"
)

/*
These unit tests use mocks to simulate chaincode-api & fabric interactions
The mocks are generated using counterfeiter directives in the comments (starting with "go:generate counterfeiter")
All files in mocks/* are generated by running following, in the directory with your directive:
	`go generate`
*/

//go:generate counterfeiter -o mocks/transaction.go -fake-name TransactionContext . transactionContext
type transactionContext interface {
	contractapi.TransactionContextInterface
}

//go:generate counterfeiter -o mocks/chaincodestub.go -fake-name ChaincodeStub . chaincodeStub
type chaincodeStub interface {
	shim.ChaincodeStubInterface
}

//go:generate counterfeiter -o mocks/statequeryiterator.go -fake-name StateQueryIterator . stateQueryIterator
type stateQueryIterator interface {
	shim.StateQueryIteratorInterface
}

//go:generate counterfeiter -o mocks/clientIdentity.go -fake-name ClientIdentity . clientIdentity
type clientIdentity interface {
	cid.ClientIdentity
}

const assetCollectionName = "assetCollection"
const transferAgreementObjectType = "transferAgreement"
const myOrg1Msp = "Org1Testmsp"
const myOrg1Clientid = "myOrg1Userid"
const myOrg1PrivCollection = "Org1TestmspPrivateCollection"
const myOrg2Msp = "Org2Testmsp"
const myOrg2Clientid = "myOrg2Userid"
const myOrg2PrivCollection = "Org2TestmspPrivateCollection"

type assetTransientInput struct {
	Type           string `json:"objectType"`
	ID             string `json:"assetID"`
	Texts          string `json:"color"`
	Size           int    `json:"size"`
	AppraisedValue int    `json:"appraisedValue"`
}

type assetTransferTransientInput struct {
	ID       string `json:"assetID"`
	BuyerMSP string `json:"buyerMSP"`
}

func TestCreateAssetBadInput(t *testing.T) {
	transactionContext, chaincodeStub := prepMocksAsOrg1()
	assetTransferCC := chaincode.SmartContract{}

	// No transient map
	err := assetTransferCC.CreateAsset(transactionContext)
	require.EqualError(t, err, "asset not found in the transient map input")

	// transient map with incomplete asset data
	assetPropMap := map[string][]byte{
		"asset_properties": []byte("ill formatted property"),
	}
	chaincodeStub.GetTransientReturns(assetPropMap, nil)
	err = assetTransferCC.CreateAsset(transactionContext)
	require.Error(t, err, "Expected error: transient map with incomplete asset data")
	require.Contains(t, err.Error(), "failed to unmarshal JSON")

	testAsset := &assetTransientInput{
		Type: "testfulasset",
	}
	setReturnAssetPropsInTransientMap(t, chaincodeStub, testAsset)
	err = assetTransferCC.CreateAsset(transactionContext)
	require.EqualError(t, err, "assetID field must be a non-empty string")

	testAsset = &assetTransientInput{
		ID:    "id1",
		Texts: "gray",
	}
	setReturnAssetPropsInTransientMap(t, chaincodeStub, testAsset)
	err = assetTransferCC.CreateAsset(transactionContext)
	require.EqualError(t, err, "objectType field must be a non-empty string")

	// case when asset exists, GetPrivateData returns a valid data from ledger
	testAsset = &assetTransientInput{
		ID:             "id1",
		Type:           "testfulasset",
		Texts:          "gray",
		Size:           7,
		AppraisedValue: 500,
	}
	setReturnAssetPropsInTransientMap(t, chaincodeStub, testAsset)
	chaincodeStub.GetPrivateDataReturns([]byte{}, nil)
	err = assetTransferCC.CreateAsset(transactionContext)
	require.EqualError(t, err, "this asset already exists: id1")
}

func TestCreateAssetSuccessful(t *testing.T) {
	transactionContext, chaincodeStub := prepMocksAsOrg1()
	assetTransferCC := chaincode.SmartContract{}
	testAsset := &assetTransientInput{
		ID:             "id1",
		Type:           "testfulasset",
		Texts:          "gray",
		Size:           7,
		AppraisedValue: 500,
	}
	setReturnAssetPropsInTransientMap(t, chaincodeStub, testAsset)
	err := assetTransferCC.CreateAsset(transactionContext)
	require.NoError(t, err)
	// Validate PutPrivateData calls
	calledCollection, calledId, _ := chaincodeStub.PutPrivateDataArgsForCall(0)
	require.Equal(t, assetCollectionName, calledCollection)
	require.Equal(t, "id1", calledId)

	expectedPrivateDetails := &chaincode.AssetPrivateDetails{
		ID:             "id1",
		AppraisedValue: 500,
	}
	assetBytes, err := json.Marshal(expectedPrivateDetails)
	calledCollection, calledId, calledAssetBytes := chaincodeStub.PutPrivateDataArgsForCall(1)
	require.Equal(t, myOrg1PrivCollection, calledCollection)
	require.Equal(t, "id1", calledId)
	require.Equal(t, assetBytes, calledAssetBytes)
}

func TestAgreeToTransferBadInput(t *testing.T) {
	transactionContext, chaincodeStub := prepMocksAsOrg1()
	assetTransferCC := chaincode.SmartContract{}

	assetPrivDetail := &chaincode.AssetPrivateDetails{
		ID: "id1",
		// no AppraisedValue
	}
	setReturnAssetPrivateDetailsInTransientMap(t, chaincodeStub, assetPrivDetail)
	origAsset := chaincode.Asset{
		ID:    "id1",
		Type:  "testfulasset",
		Texts: "gray",
		Size:  7,
		Owner: myOrg1Clientid,
	}
	setReturnPrivateDataInStub(t, chaincodeStub, &origAsset)

	err := assetTransferCC.AgreeToTransfer(transactionContext)
	require.EqualError(t, err, "appraisedValue field must be a positive integer")

	assetPrivDetail = &chaincode.AssetPrivateDetails{
		// no ID
		AppraisedValue: 500,
	}
	setReturnAssetPrivateDetailsInTransientMap(t, chaincodeStub, assetPrivDetail)
	err = assetTransferCC.AgreeToTransfer(transactionContext)
	require.EqualError(t, err, "assetID field must be a non-empty string")

	assetPrivDetail = &chaincode.AssetPrivateDetails{
		ID:             "id1",
		AppraisedValue: 500,
	}
	setReturnAssetPrivateDetailsInTransientMap(t, chaincodeStub, assetPrivDetail)
	// asset does not exist
	setReturnPrivateDataInStub(t, chaincodeStub, nil)
	err = assetTransferCC.AgreeToTransfer(transactionContext)
	require.EqualError(t, err, "id1 does not exist")
}

func TestAgreeToTransferSuccessful(t *testing.T) {
	transactionContext, chaincodeStub := prepMocksAsOrg1()
	assetTransferCC := chaincode.SmartContract{}
	assetPrivDetail := &chaincode.AssetPrivateDetails{
		ID:             "id1",
		AppraisedValue: 500,
	}
	setReturnAssetPrivateDetailsInTransientMap(t, chaincodeStub, assetPrivDetail)
	origAsset := chaincode.Asset{
		ID:    "id1",
		Type:  "testfulasset",
		Texts: "gray",
		Size:  7,
		Owner: myOrg1Clientid,
	}
	setReturnPrivateDataInStub(t, chaincodeStub, &origAsset)
	chaincodeStub.CreateCompositeKeyReturns(transferAgreementObjectType+"id1", nil)
	err := assetTransferCC.AgreeToTransfer(transactionContext)
	require.NoError(t, err)

	expectedDataBytes, err := json.Marshal(assetPrivDetail)
	calledCollection, calledId, calledWithDataBytes := chaincodeStub.PutPrivateDataArgsForCall(0)
	require.Equal(t, myOrg1PrivCollection, calledCollection)
	require.Equal(t, "id1", calledId)
	require.Equal(t, expectedDataBytes, calledWithDataBytes)

	calledCollection, calledId, calledWithDataBytes = chaincodeStub.PutPrivateDataArgsForCall(1)
	require.Equal(t, assetCollectionName, calledCollection)
	require.Equal(t, transferAgreementObjectType+"id1", calledId)
	require.Equal(t, []byte(myOrg1Clientid), calledWithDataBytes)
}
func TestTransferAssetBadInput(t *testing.T) {
	transactionContext, chaincodeStub := prepMocksAsOrg1()
	assetTransferCC := chaincode.SmartContract{}

	assetNewOwner := &assetTransferTransientInput{
		ID:       "id1",
		BuyerMSP: "",
	}
	setReturnAssetOwnerInTransientMap(t, chaincodeStub, assetNewOwner)
	setReturnPrivateDataInStub(t, chaincodeStub, &chaincode.Asset{})
	err := assetTransferCC.TransferAsset(transactionContext)
	require.EqualError(t, err, "buyerMSP field must be a non-empty string")

	assetNewOwner = &assetTransferTransientInput{
		ID:       "id1",
		BuyerMSP: myOrg2Msp,
	}
	setReturnAssetOwnerInTransientMap(t, chaincodeStub, assetNewOwner)
	// asset does not exist
	setReturnPrivateDataInStub(t, chaincodeStub, nil)
	err = assetTransferCC.TransferAsset(transactionContext)
	require.EqualError(t, err, "id1 does not exist")
}

func TestTransferAssetSuccessful(t *testing.T) {
	transactionContext, chaincodeStub := prepMocksAsOrg1()
	assetTransferCC := chaincode.SmartContract{}
	assetNewOwner := &assetTransferTransientInput{
		ID:       "id1",
		BuyerMSP: myOrg2Msp,
	}
	setReturnAssetOwnerInTransientMap(t, chaincodeStub, assetNewOwner)
	origAsset := chaincode.Asset{
		ID:    "id1",
		Type:  "testfulasset",
		Texts: "gray",
		Size:  7,
		Owner: myOrg1Clientid,
	}
	setReturnPrivateDataInStub(t, chaincodeStub, &origAsset)
	// to ensure we pass data hash verification
	chaincodeStub.GetPrivateDataHashReturns([]byte("datahash"), nil)
	// to ensure that ReadTransferAgreement call returns org2 client ID
	chaincodeStub.GetPrivateDataReturnsOnCall(1, []byte(myOrg2Clientid), nil)
	chaincodeStub.CreateCompositeKeyReturns(transferAgreementObjectType+"id1", nil)

	err := assetTransferCC.TransferAsset(transactionContext)
	require.NoError(t, err)
	// Validate PutPrivateData calls
	expectedNewAsset := origAsset
	expectedNewAsset.Owner = myOrg2Clientid
	expectedNewAssetBytes, err := json.Marshal(expectedNewAsset)
	require.NoError(t, err)
	calledCollection, calledId, calledWithAssetBytes := chaincodeStub.PutPrivateDataArgsForCall(0)
	require.Equal(t, assetCollectionName, calledCollection)
	require.Equal(t, "id1", calledId)
	require.Equal(t, expectedNewAssetBytes, calledWithAssetBytes)
	calledCollection, calledId = chaincodeStub.DelPrivateDataArgsForCall(0)
	require.Equal(t, myOrg1PrivCollection, calledCollection)
	require.Equal(t, "id1", calledId)

	calledCollection, calledId = chaincodeStub.DelPrivateDataArgsForCall(1)
	require.Equal(t, assetCollectionName, calledCollection)
	require.Equal(t, transferAgreementObjectType+"id1", calledId)

}

func TestTransferAssetByNonOwner(t *testing.T) {
	transactionContext, chaincodeStub := prepMocksAsOrg1()
	assetTransferCC := chaincode.SmartContract{}
	assetNewOwner := &assetTransferTransientInput{
		ID:       "id1",
		BuyerMSP: myOrg1Msp,
	}
	setReturnAssetOwnerInTransientMap(t, chaincodeStub, assetNewOwner)
	// Try to transfer asset owned by Org2
	org2Asset := chaincode.Asset{
		ID:    "id1",
		Type:  "testfulasset",
		Texts: "gray",
		Size:  7,
		Owner: myOrg2Clientid,
	}
	setReturnPrivateDataInStub(t, chaincodeStub, &org2Asset)
	err := assetTransferCC.TransferAsset(transactionContext)
	require.EqualError(t, err, "failed transfer verification: error: submitting client identity does not own asset")
}

func TestTransferAssetWithoutAnAgreement(t *testing.T) {
	transactionContext, chaincodeStub := prepMocksAsOrg1()
	assetTransferCC := chaincode.SmartContract{}
	assetNewOwner := &assetTransferTransientInput{
		ID:       "id1",
		BuyerMSP: myOrg1Msp,
	}
	setReturnAssetOwnerInTransientMap(t, chaincodeStub, assetNewOwner)
	orgAsset := chaincode.Asset{
		ID:    "id1",
		Type:  "testfulasset",
		Texts: "gray",
		Size:  7,
		Owner: myOrg1Clientid,
	}
	setReturnPrivateDataInStub(t, chaincodeStub, &orgAsset)
	// to ensure we pass data hash verification
	chaincodeStub.GetPrivateDataHashReturns([]byte("datahash"), nil)
	chaincodeStub.CreateCompositeKeyReturns(transferAgreementObjectType+"id1", nil)
	// ReadTransferAgreement call returns no buyer client ID
	chaincodeStub.GetPrivateDataReturnsOnCall(1, []byte{}, nil)

	err := assetTransferCC.TransferAsset(transactionContext)
	require.EqualError(t, err, "BuyerID not found in TransferAgreement for id1")
}

func TestTransferAssetNonMatchingAppraisalValue(t *testing.T) {
	transactionContext, chaincodeStub := prepMocksAsOrg1()
	assetTransferCC := chaincode.SmartContract{}
	assetNewOwner := &assetTransferTransientInput{
		ID:       "id1",
		BuyerMSP: myOrg2Msp,
	}
	setReturnAssetOwnerInTransientMap(t, chaincodeStub, assetNewOwner)

	orgAsset := chaincode.Asset{
		ID:    "id1",
		Type:  "testfulasset",
		Texts: "gray",
		Size:  7,
		Owner: myOrg1Clientid,
	}
	setReturnPrivateDataInStub(t, chaincodeStub, &orgAsset)
	chaincodeStub.CreateCompositeKeyReturns(transferAgreementObjectType+"id1", nil)
	// data hash different in each collection
	chaincodeStub.GetPrivateDataHashReturnsOnCall(0, []byte("datahash1"), nil)
	chaincodeStub.GetPrivateDataHashReturnsOnCall(1, []byte("datahash2"), nil)

	err := assetTransferCC.TransferAsset(transactionContext)
	require.Error(t, err, "Expected failed hash verification")
	require.Contains(t, err.Error(), "failed transfer verification: hash for appraised value")
}

func prepMocksAsOrg1() (*mocks.TransactionContext, *mocks.ChaincodeStub) {
	return prepMocks(myOrg1Msp, myOrg1Clientid)
}
func prepMocksAsOrg2() (*mocks.TransactionContext, *mocks.ChaincodeStub) {
	return prepMocks(myOrg2Msp, myOrg2Clientid)
}
func prepMocks(orgMSP, clientId string) (*mocks.TransactionContext, *mocks.ChaincodeStub) {
	chaincodeStub := &mocks.ChaincodeStub{}
	transactionContext := &mocks.TransactionContext{}
	transactionContext.GetStubReturns(chaincodeStub)

	clientIdentity := &mocks.ClientIdentity{}
	clientIdentity.GetMSPIDReturns(orgMSP, nil)
	clientIdentity.GetIDReturns(base64.StdEncoding.EncodeToString([]byte(clientId)), nil)
	// set matching msp ID using peer shim env variable
	os.Setenv("CORE_PEER_LOCALMSPID", orgMSP)
	transactionContext.GetClientIdentityReturns(clientIdentity)
	return transactionContext, chaincodeStub
}

func setReturnAssetPrivateDetailsInTransientMap(t *testing.T, chaincodeStub *mocks.ChaincodeStub, assetPrivDetail *chaincode.AssetPrivateDetails) []byte {
	assetOwnerBytes := []byte{}
	if assetPrivDetail != nil {
		var err error
		assetOwnerBytes, err = json.Marshal(assetPrivDetail)
		require.NoError(t, err)
	}
	assetPropMap := map[string][]byte{
		"asset_value": assetOwnerBytes,
	}
	chaincodeStub.GetTransientReturns(assetPropMap, nil)
	return assetOwnerBytes
}

func setReturnAssetOwnerInTransientMap(t *testing.T, chaincodeStub *mocks.ChaincodeStub, assetOwner *assetTransferTransientInput) []byte {
	assetOwnerBytes := []byte{}
	if assetOwner != nil {
		var err error
		assetOwnerBytes, err = json.Marshal(assetOwner)
		require.NoError(t, err)
	}
	assetPropMap := map[string][]byte{
		"asset_owner": assetOwnerBytes,
	}
	chaincodeStub.GetTransientReturns(assetPropMap, nil)
	return assetOwnerBytes
}

func setReturnAssetPropsInTransientMap(t *testing.T, chaincodeStub *mocks.ChaincodeStub, testAsset *assetTransientInput) []byte {
	assetBytes := []byte{}
	if testAsset != nil {
		var err error
		assetBytes, err = json.Marshal(testAsset)
		require.NoError(t, err)
	}
	assetPropMap := map[string][]byte{
		"asset_properties": assetBytes,
	}
	chaincodeStub.GetTransientReturns(assetPropMap, nil)
	return assetBytes
}

func setReturnPrivateDataInStub(t *testing.T, chaincodeStub *mocks.ChaincodeStub, testAsset *chaincode.Asset) []byte {
	if testAsset == nil {
		chaincodeStub.GetPrivateDataReturns(nil, nil)
		return nil
	} else {
		var err error
		assetBytes, err := json.Marshal(testAsset)
		require.NoError(t, err)
		chaincodeStub.GetPrivateDataReturns(assetBytes, nil)
		return assetBytes
	}
}

func setReturnAssetPrivateDetailsInStub(t *testing.T, chaincodeStub *mocks.ChaincodeStub, testAsset *chaincode.AssetPrivateDetails) []byte {
	if testAsset == nil {
		chaincodeStub.GetPrivateDataReturns(nil, nil)
		return nil
	} else {
		var err error
		assetBytes, err := json.Marshal(testAsset)
		require.NoError(t, err)
		chaincodeStub.GetPrivateDataReturns(assetBytes, nil)
		return assetBytes
	}
}
