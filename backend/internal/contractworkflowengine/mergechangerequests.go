package contractworkflowengine

import (
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/db"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"
)

func MergeChangeRequests(tx *sqlx.Tx, cRepo db.ContractRepo, nRepo db.NegotiationRepo, did string, contractVersion *int) error {
	//changeRequests, err := nRepo.ReadAllAcceptedByContractDIDAndVersion(tx, did, contractVersion)
	//if err != nil {
	//	return err
	//}

	//for _, changeRequest := range changeRequests {
	//	changes, err := changeRequestToMap(changeRequest.ChangeRequest)
	//	if err != nil {
	//		return err
	//	}
	//}

	return nil
}

func changeRequestToMap(req *datatype.JSON) (map[string]*datatype.JSON, error) {
	if req == nil {
		return nil, nil
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(*req, &rawMap); err != nil {
		return nil, fmt.Errorf("could not unmarshal change request: %w", err)
	}

	result := make(map[string]*datatype.JSON)
	for key, value := range rawMap {
		v := datatype.JSON(value)
		result[key] = &v
	}

	return result, nil
}
