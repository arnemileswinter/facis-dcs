Feature: QA fixes preserve archive, classification, organization scope and evidence

  @REQ-qa-fixes-abschluss-AC3 @DCS-FR-CSA-12 @DCS-IR-CSA-06
  Scenario: Observer reads the selected immutable archive version after the live record changes
    Given a dedicated QA contract "QA Frozen Contract" has reached SIGNED
    And the Observer remembers the archived version of QA contract "QA Frozen Contract"
    And the live QA contract "QA Frozen Contract" advances to a newer version through the database fixture seam
    When the Observer opens the remembered archived version of QA contract "QA Frozen Contract"
    Then the QA archive detail is the unchanged snapshot for contract "QA Frozen Contract" with matching identity and hash
    When the Observer requests a non-archived version of QA contract "QA Frozen Contract"
    Then the response status is 404

  @REQ-qa-fixes-abschluss-AC4 @DCS-FR-TR-27 @DCS-IR-TR-02 @DCS-IR-SI-01
  Scenario Outline: Explicit party classification survives editing and copying and combines with local and catalogue text filters
    Given QA template "QA Match Classification" is created as "<template_type>" with party classification "<party_type>"
    And QA template "QA Match Other Party" is created as "<template_type>" with party classification "<other_party>"
    And QA template "QA Unrelated Name" is created as "<template_type>" with party classification "<party_type>"
    When the QA template "QA Match Classification" is edited without a party field and copied
    Then QA template "QA Match Classification", its history and its copy retain "<template_type>" and "<party_type>"
    When the QA local search combines name "QA Match" and party classification "<party_type>"
    Then the QA search includes template "QA Match Classification" and excludes "QA Match Other Party" and "QA Unrelated Name"
    When QA templates "QA Match Classification", "QA Match Other Party" and "QA Unrelated Name" are published to the catalogue
    And the QA catalogue search combines name "QA Match" and party classification "<party_type>"
    Then the QA search includes template "QA Match Classification" and excludes "QA Match Other Party" and "QA Unrelated Name"

    Examples:
      | template_type     | party_type   | other_party  |
      | CONTRACT_TEMPLATE | SINGLE_PARTY | MULTI_PARTY  |
      | CONTRACT_TEMPLATE | MULTI_PARTY  | SINGLE_PARTY |
      | COMPONENT         | SINGLE_PARTY | MULTI_PARTY  |
      | COMPONENT         | MULTI_PARTY  | SINGLE_PARTY |

  @REQ-qa-fixes-abschluss-AC5 @DCS-FR-CWE-07 @DCS-NFR-SEC-03
  Scenario: Dashboard lists and open tasks are shared within an organization and hide unrelated organizations
    Given a dedicated QA contract "QA Visible Org Contract" has reached SIGNED
    And a dedicated QA contract "QA Foreign Org Contract" has reached SIGNED
    And QA contracts "QA Visible Org Contract" and "QA Foreign Org Contract" have open local review, approval and negotiation tasks through the database fixture seam
    When the QA organization of contract "QA Visible Org Contract" retrieves the contract dashboard
    Then the QA dashboard includes contract "QA Visible Org Contract" and its open tasks but excludes "QA Foreign Org Contract" and its tasks
    And the QA organization of contract "QA Visible Org Contract" sees it in contract, archive and signing searches but cannot see "QA Foreign Org Contract"
    When a QA Manager from the organization of "QA Visible Org Contract" requests "/contract/retrieve/{did}" for contract "QA Visible Org Contract"
    Then the response status is 200
    And the QA privileged readers can retrieve contract "QA Foreign Org Contract" from an unrelated organization

  @REQ-qa-fixes-abschluss-AC5 @DCS-FR-CWE-07 @DCS-NFR-SEC-03
  Scenario Outline: Indirect contract reads reject an unrelated organization with an otherwise valid role
    Given a dedicated QA contract "QA Authorized Org" has reached SIGNED
    And a dedicated QA contract "QA Indirect Foreign" has reached SIGNED
    When a QA Manager from the organization of "QA Authorized Org" requests "<path>" for contract "QA Indirect Foreign"
    Then the response status is 403

    Examples:
      | path                        |
      | /contract/retrieve/{did}    |
      | /contract/history/{did}     |
      | /signature/retrieve/{did}   |
      | /pdf/export/contract/{did}  |
      | /contract/export/{did}      |

  # The archive deletion erases only a contract that is no longer in force, so
  # the signed contract is terminated before the deletion.
  @REQ-qa-fixes-abschluss-AC6 @DCS-IR-CSA-02 @DCS-IR-CWE-12 @DCS-NFR-SEC-13
  Scenario: Contract Manager retrieves exact uploaded evidence until archive deletion shreds its key
    Given a dedicated QA contract "QA Evidence Contract" has reached SIGNED
    And the QA evidence file "qa-evidence.bin" contains hexadecimal bytes "00ff0102030451612045766964656e63650ac3a4"
    When the QA Contract Manager uploads the file to contract "QA Evidence Contract" with description "Confidential QA evidence description"
    And the QA Contract Manager downloads that evidence from contract "QA Evidence Contract"
    Then the QA evidence has the original file bytes, metadata and server SHA256 for contract "QA Evidence Contract"
    When the QA Contract Manager terminates contract "QA Evidence Contract"
    And the QA Archive Manager deletes contract "QA Evidence Contract" with an erasure justification
    Then the response status is 200
    And the QA contract "QA Evidence Contract" has a shredded key and its evidence is unreadable
