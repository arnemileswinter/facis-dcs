# Export a readable audit report

## Purpose

The export returns the newest saved audit run for the selected scope and
contract DID that you may read in full. A newer run that contains contracts you
may not read does not block the export. Your current read rights decide, not
the person who started the run. The export uses the saved results and does not
run a new audit.

## Roles

- Log in as **Auditor** or **Archive Manager**. Without the Auditor role, an
  Archive Manager can export the **Archive** scope only.
- An Archive Manager must be able to read every contract in the selected
  report. A report is never shortened to the contracts you may read.

## Steps

1. Open the audit view and, if shown, the **Compliance Audit** tab.
2. Select the scope under **Scope**. Optionally enter a contract DID under
   **DID (optional)**. Enter a reason under **Audit justification**.
3. Click **Execute Audit**. After a successful audit for the current selection,
   the download becomes available.
4. Under **Download report**, select **JSON**, **CSV** or **PDF**. The download
   uses the newest saved run that you may read in full. An audit that another
   person runs in the meantime can change which run that is.

## Errors

- **Download disabled:** an audit or a download is still running, the reason is
  missing, or no audit has run successfully for the current selection.
- **Audit report could not be generated.:** the export failed. If you may not
  read any matching report in full, no partial report is returned. If no
  matching run is saved, no report is available.
- If a required decryption key was erased, that report is no longer available.
  Damaged saved data or read errors also stop the export. In these cases, no
  older report is returned instead.
