package userrole

// UserRole names mirror the OAuth2 scopes declared in `backend/design`
// (see Scope() calls in the design package).
type UserRole string

const (
	TemplateCreator  UserRole = "Template Creator"
	TemplateReviewer UserRole = "Template Reviewer"
	TemplateApprover UserRole = "Template Approver"
	TemplateManager  UserRole = "Template Manager"

	ContractCreator  UserRole = "Contract Creator"
	ContractReviewer UserRole = "Contract Reviewer"
	ContractApprover UserRole = "Contract Approver"
	ContractManager  UserRole = "Contract Manager"
	ContractSigner   UserRole = "Contract Signer"
	ContractObserver UserRole = "Contract Observer"

	ArchiveManager      UserRole = "Archive Manager"
	Auditor             UserRole = "Auditor"
	SystemAdministrator UserRole = "System Administrator"
	ComplianceOfficer   UserRole = "Compliance Officer"

	SystemContractCreator  UserRole = "Sys. Contract Creator"
	SystemContractReviewer UserRole = "Sys. Contract Reviewer"
	SystemContractApprover UserRole = "Sys. Contract Approver"
	SystemContractManager  UserRole = "Sys. Contract Manager"
	SystemContractSigner   UserRole = "Sys. Contract Signer"
)

func (r UserRole) IsValid() bool {
	switch r {
	case TemplateCreator, TemplateReviewer, TemplateApprover, TemplateManager,
		ContractCreator, ContractReviewer, ContractApprover, ContractManager,
		ContractSigner, ContractObserver, ArchiveManager, Auditor,
		SystemAdministrator, ComplianceOfficer, SystemContractCreator,
		SystemContractReviewer, SystemContractApprover, SystemContractManager,
		SystemContractSigner:
		return true
	}
	return false
}

func All() []UserRole {
	return []UserRole{
		TemplateCreator, TemplateReviewer, TemplateApprover, TemplateManager,
		ContractCreator, ContractReviewer, ContractApprover, ContractManager,
		ContractSigner, ContractObserver, ArchiveManager, Auditor,
		SystemAdministrator, ComplianceOfficer, SystemContractCreator,
		SystemContractReviewer, SystemContractApprover, SystemContractManager, SystemContractSigner,
	}
}

func (r UserRole) String() string { return string(r) }

func (r UserRole) IsSystemRole() bool {
	switch r {
	case SystemContractCreator, SystemContractReviewer, SystemContractApprover,
		SystemContractManager, SystemContractSigner:
		return true
	}
	return false
}

func (r UserRole) IsHumanRole() bool {
	return r.IsValid() && !r.IsSystemRole()
}
