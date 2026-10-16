package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	archive "digital-contracting-service/gen/contract_storage_archive"
	"digital-contracting-service/internal/base/artifactstore"
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/command"
)

func (s *contractStorageArchivesrvc) Detail(ctx context.Context, p *archive.DetailPayload) (any, error) {
	if err := authorizeContractReadDocument(ctx, s.DB, s.DIDDocument, p.Did); err != nil {
		return nil, err
	}
	var entry struct {
		CID  string `db:"snapshot_cid"`
		Hash string `db:"content_hash"`
	}
	err := s.DB.GetContext(ctx, &entry, `SELECT snapshot_cid, content_hash FROM contract_archive_entries WHERE did=$1 AND contract_version=$2 AND deleted_at IS NULL AND archive_status <> 'DELETED'`, p.Did, p.ContractVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, archive.MakeNotFound(err)
	}
	if err != nil {
		return nil, archive.MakeInternalError(err)
	}
	if entry.CID == "" || s.ATrailReader.Artifacts == nil {
		return nil, archive.MakeNotFound(errors.New("archive snapshot unavailable"))
	}
	raw, err := s.ATrailReader.Artifacts.Get(ctx, artifactstore.ContractScope(p.Did), entry.CID)
	if err != nil {
		return nil, archive.MakeNotFound(errors.New("archive snapshot unavailable"))
	}
	hash, err := command.HashArchiveSnapshot(datatype.JSON(raw))
	if err != nil || hash != entry.Hash {
		return nil, archive.MakeInternalError(errors.New("archive snapshot integrity mismatch"))
	}
	var identity struct {
		DID     string `json:"did"`
		Version int    `json:"contract_version"`
	}
	if err := json.Unmarshal(raw, &identity); err != nil || identity.DID != p.Did || identity.Version != p.ContractVersion {
		return nil, archive.MakeInternalError(fmt.Errorf("archive snapshot identity mismatch"))
	}
	var snapshot any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, archive.MakeInternalError(err)
	}
	return snapshot, nil
}
