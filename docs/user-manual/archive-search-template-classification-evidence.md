# Find archived contracts, classify templates and attach evidence

These features help you find the right archived version, distinguish template
party arrangements, and keep supporting files with a contract.

## Roles and prerequisites

- An **Observer** can search and read accessible archived contracts. Archive
  management requires the corresponding management role.
- Use your existing template creation/editing permissions to classify templates.
- A **Contract Manager** can attach evidence and open the contract's audit history tab.
  Your role alone does not grant access to unrelated organizations' contracts.

## Find an archived version

1. Open the archive dashboard and find **Archived contracts**.
2. Enter filters such as **DID**, **Party**, **State**, **Version**, **Valid from**
   or **Valid until**, then select **Search archive**.
3. Open the matching result. Check its version and metadata before reading the
   document preview or machine-readable content. This is the archived version,
   displayed read-only.
4. Select **Back to archive** to return or **Reset search** to clear filters.

A failed request displays an error instead of an old document. Use **Retry**
when available. Missing, deleted or no longer readable versions cannot be
opened. An empty result means no accessible archived entry matched the filters.

## Classify a template

1. Open the template details while creating or editing a template.
2. Select **Single-party** or **Multi-party** under **Party type**. This choice
   is separate from whether the template represents a contract or a component.
3. Complete the other required details and save. Creating a template without a
   party type shows a validation error and does not save.
4. Use the party-type filter in the template repository or federated catalogue
   together with text search to narrow results.

## Attach and download contract evidence

1. As a Contract Manager, open an accessible contract's **Audit History** tab.
2. Under **Contract evidence**, select **Store evidence**.
3. Choose an **Evidence file** and enter an **Evidence description**.
4. Select **Save evidence** and wait for completion. The new item appears in
   the evidence list; repeated submission is disabled while saving.
5. Select an item's **Download** button to retrieve its original file.

**Cancel** closes the form without saving. A failed save keeps the inputs for
retry; a failed list load offers **Retry evidence**. Invalid input, configured
size limits, a changed contract or insufficient access can prevent saving.
Download failures display an error. Evidence cannot be read after its contract's
encryption keys have been destroyed. Changing contracts clears the previous
contract's evidence view and unfinished input.
