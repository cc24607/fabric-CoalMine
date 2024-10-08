package chaincode

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
	"strconv"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// SmartContract provides functions for managing an Asset
type SmartContract struct {
	contractapi.Contract
}

// Asset describes basic details of what makes up a simple asset
// Insert struct field in alphabetic order => to achieve determinism across languages
// golang keeps the order when marshal to json but doesn't order automatically
type Asset struct {
	AppraisedValue int    `json:"AppraisedValue"`
	Texts          string `json:"Texts"`
	ID             string `json:"ID"`
	Owner          string `json:"Owner"`
	Size           int    `json:"Size"`
}

// InitLedger adds a base set of assets to the ledger
func (s *SmartContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	assets := []Asset{
		{ID: "asset1", Texts: "empty string", Size: 5, Owner: "Tomoko", AppraisedValue: 300},
		{ID: "asset2", Texts: "chaincode", Size: 5, Owner: "Brad", AppraisedValue: 400},
		{ID: "asset3", Texts: "requirements", Size: 10, Owner: "Jin Soo", AppraisedValue: 500},
		{ID: "asset4", Texts: "auto-incrementing integer", Size: 10, Owner: "Max", AppraisedValue: 600},
		{ID: "asset5", Texts: "example", Size: 15, Owner: "Adriana", AppraisedValue: 700},
		{ID: "asset6", Texts: "information", Size: 15, Owner: "Michel", AppraisedValue: 800},
	}

	for _, asset := range assets {
		assetJSON, err := json.Marshal(asset)
		if err != nil {
			return err
		}

		err = ctx.GetStub().PutState(asset.ID, assetJSON)
		if err != nil {
			return fmt.Errorf("failed to put to world state. %v", err)
		}
	}

	return nil
}

// CreateAsset issues a new asset to the world state with given details.
func (s *SmartContract) CreateAsset(ctx contractapi.TransactionContextInterface, id string, color string, size int, owner string, appraisedValue int) error {
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("the asset %s already exists", id)
	}

	asset := Asset{
		ID:             id,
		Texts:          color,
		Size:           size,
		Owner:          owner,
		AppraisedValue: appraisedValue,
	}
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(id, assetJSON)
}

// ReadAsset returns the asset stored in the world state with given id.
func (s *SmartContract) ReadAsset(ctx contractapi.TransactionContextInterface, id string) (*Asset, error) {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if assetJSON == nil {
		return nil, fmt.Errorf("the asset %s does not exist", id)
	}

	var asset Asset
	err = json.Unmarshal(assetJSON, &asset)
	if err != nil {
		return nil, err
	}

	return &asset, nil
}

// UpdateAsset updates an existing asset in the world state with provided parameters.
func (s *SmartContract) UpdateAsset(ctx contractapi.TransactionContextInterface, id string, color string, size int, owner string, appraisedValue int) error {
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("the asset %s does not exist", id)
	}

	// overwriting original asset with new asset
	asset := Asset{
		ID:             id,
		Texts:          color,
		Size:           size,
		Owner:          owner,
		AppraisedValue: appraisedValue,
	}
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(id, assetJSON)
}

// DeleteAsset deletes an given asset from the world state.
func (s *SmartContract) DeleteAsset(ctx contractapi.TransactionContextInterface, id string) error {
	exists, err := s.AssetExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("the asset %s does not exist", id)
	}

	return ctx.GetStub().DelState(id)
}

// AssetExists returns true when asset with given ID exists in world state
func (s *SmartContract) AssetExists(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}

	return assetJSON != nil, nil
}

// TransferAsset updates the owner field of asset with given id in world state, and returns the old owner.
func (s *SmartContract) TransferAsset(ctx contractapi.TransactionContextInterface, id string, newOwner string) (string, error) {
	asset, err := s.ReadAsset(ctx, id)
	if err != nil {
		return "", err
	}

	oldOwner := asset.Owner
	asset.Owner = newOwner

	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return "", err
	}

	err = ctx.GetStub().PutState(id, assetJSON)
	if err != nil {
		return "", err
	}

	return oldOwner, nil
}

// GetAllAssets returns all assets found in world state
func (s *SmartContract) GetAllAssets(ctx contractapi.TransactionContextInterface) ([]*Asset, error) {
	// range query with empty string for startKey and endKey does an
	// open-ended query of all assets in the chaincode namespace.
	resultsIterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var assets []*Asset
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var asset Asset
		err = json.Unmarshal(queryResponse.Value, &asset)
		if err != nil {
			return nil, err
		}
		assets = append(assets, &asset)
	}

	return assets, nil
}

// CalculateGapScoreWithDecay calculates the gap score using an exponential decay function.
// It is assumed that actualValue, standardValue, weightFactor, dataCreationTime, and lambda
// are provided as arguments to the transaction.
func (s *SmartContract) CalculateGapScoreWithDecay(ctx contractapi.TransactionContextInterface, id string, actualValue, standardValue, weightFactor float64, dataCreationTimeString, lambdaString string) (float64, error) {
	// Get current time from the context or use the system's current time
	currentTime := time.Now() // In a real blockchain scenario, you may want to use a timestamp from the transaction context.

	// Parse the data creation time from the input string
	dataCreationTime, err := time.Parse(time.RFC3339, dataCreationTimeString)
	if err != nil {
		return 0, fmt.Errorf("failed to parse data creation time: %v", err)
	}

	// Parse the lambda value from the input string
	lambda, err := strconv.ParseFloat(lambdaString, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse lambda: %v", err)
	}

	// Calculate the time difference in seconds
	deltaT := currentTime.Sub(dataCreationTime).Seconds()

	// Calculate the exponential decay factor
	decayFactor := math.Exp(-lambda * deltaT)

	// Calculate the absolute value of the gap
	gap := math.Abs(actualValue - standardValue)

	// Calculate the relative gap as a proportion of the standard value
	relativeGap := gap / standardValue

	// Apply the weight factor and decay factor
	gapScore := relativeGap * weightFactor * decayFactor

	return gapScore, nil
}


