package command

import (
	"fmt"
	"time"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
)

// ArchiveManagerStoreSource marks an archive entry an Archive Manager stored
// (DCS-IR-CSA-02), as opposed to the entry the signing workflow writes.
const ArchiveManagerStoreSource = "ARCHIVE_MANAGER_STORE"

// archivableStates are the final states an Archive Manager can archive a
// contract in.
var archivableStates = map[string]bool{
	contractstate.Signed.String():     true,
	contractstate.Terminated.String(): true,
	contractstate.Expired.String():    true,
}

// BuildManualArchiveEntry freezes the current version of a SIGNED, TERMINATED
// or EXPIRED contract for an archive entry an Archive Manager stores. The
// entry records no signature evidence: the signing workflow is its only
// source.
func BuildManualArchiveEntry(contract *db.Contract, storedBy string) (db.ContractArchiveEntry, error) {
	if contract == nil {
		return db.ContractArchiveEntry{}, fmt.Errorf("contract is required")
	}
	if !archivableStates[contract.State] {
		return db.ContractArchiveEntry{}, fmt.Errorf("contract %s is %s; only SIGNED, TERMINATED or EXPIRED contracts can be archived", contract.DID, contract.State)
	}
	snapshot, err := buildContractSnapshot(contract)
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}
	contentHash, err := HashArchiveSnapshot(snapshot)
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}
	evidence, err := datatype.NewJSON(map[string]any{
		"source":                  ArchiveManagerStoreSource,
		"stored_by":               storedBy,
		"stored_state":            contract.State,
		"snapshot_hash_algorithm": archiveSnapshotHashAlgorithm,
	})
	if err != nil {
		return db.ContractArchiveEntry{}, err
	}
	return db.ContractArchiveEntry{
		DID:              contract.DID,
		ContractVersion:  contract.ContractVersion,
		StoredBy:         storedBy,
		StoredAt:         time.Now().UTC(),
		ContractSnapshot: snapshot,
		ContentHash:      contentHash,
		Evidence:         &evidence,
	}, nil
}
