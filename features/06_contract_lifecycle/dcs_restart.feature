# DCS-NFR-SF-01 Reset Possibility: the DCS is reset (its deployment restarted,
# so a new pod replaces the old one) in the middle of a contract workflow.
# The contract created before the reset is readable after it and continues
# its workflow on the new pod without any recovery step.

@NFR-SF-01 @DCS-NFR-SF-01
Feature: DCS restart

  @clean_db
  Scenario: A contract created before a DCS restart is read and continued after it
    Given contract "Restart Survivor Contract" has reached contract state "DRAFT"
    When the DCS deployment restarts
    Then the contract "Restart Survivor Contract" is in state "DRAFT"
    When the creator submits contract "Restart Survivor Contract" for review after the restart
    Then the contract "Restart Survivor Contract" is in state "SUBMITTED"
