package userrole

type UserRole string

const (
	// Human User Roles - Template Management
	TemplateCreator  UserRole = "TEMPLATE_CREATOR"
	TemplateReviewer UserRole = "TEMPLATE_REVIEWER"
	TemplateApprover UserRole = "TEMPLATE_APPROVER"
	TemplateManager  UserRole = "TEMPLATE_MANAGER"

	// Human User Roles - Contract Management
	ContractCreator  UserRole = "CONTRACT_CREATOR"
	ContractReviewer UserRole = "CONTRACT_REVIEWER"
	ContractApprover UserRole = "CONTRACT_APPROVER"
	ContractManager  UserRole = "CONTRACT_MANAGER"
	ContractSigner   UserRole = "CONTRACT_SIGNER"
	ContractObserver UserRole = "CONTRACT_OBSERVER"

	// Human User Roles - System Administration
	ArchiveManager     UserRole = "ARCHIVE_MANAGER"
	Auditor            UserRole = "AUDITOR"
	SystemAdmin        UserRole = "SYSTEM_ADMINISTRATOR"
	ComplianceOfficer  UserRole = "COMPLIANCE_OFFICER"
	IntegrationManager UserRole = "INTEGRATION_MANAGER"

	// Human User Roles - Process Management
	ProcessOrchestrator UserRole = "PROCESS_ORCHESTRATOR"
	Validator           UserRole = "VALIDATOR"

	// System User Roles - API/Automated
	SystemContractCreator  UserRole = "SYSTEM_CONTRACT_CREATOR"
	SystemContractReviewer UserRole = "SYSTEM_CONTRACT_REVIEWER"
	SystemContractApprover UserRole = "SYSTEM_CONTRACT_APPROVER"
	SystemContractManager  UserRole = "SYSTEM_CONTRACT_MANAGER"
	SystemContractSigner   UserRole = "SYSTEM_CONTRACT_SIGNER"
	ContractTargetSystem   UserRole = "CONTRACT_TARGET_SYSTEM"
)

// IsValid checks if the UserRole is a valid role
func (r UserRole) IsValid() bool {
	switch r {
	case TemplateCreator, TemplateReviewer, TemplateApprover, TemplateManager,
		ContractCreator, ContractReviewer, ContractApprover, ContractManager,
		ContractSigner, ContractObserver,
		ArchiveManager, Auditor, SystemAdmin, ComplianceOfficer, IntegrationManager,
		ProcessOrchestrator, Validator,
		SystemContractCreator, SystemContractReviewer, SystemContractApprover,
		SystemContractManager, SystemContractSigner, ContractTargetSystem:
		return true
	}
	return false
}

// String returns the string representation of the UserRole
func (r UserRole) String() string {
	return string(r)
}

// IsSystemRole returns true if the role is a system/automated role
func (r UserRole) IsSystemRole() bool {
	switch r {
	case SystemContractCreator, SystemContractReviewer, SystemContractApprover,
		SystemContractManager, SystemContractSigner, ContractTargetSystem:
		return true
	}
	return false
}

// IsHumanRole returns true if the role is a human user role
func (r UserRole) IsHumanRole() bool {
	return r.IsValid() && !r.IsSystemRole()
}
