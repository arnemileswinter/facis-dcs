package event

import (
	"testing"

	cweeventtype "digital-contracting-service/internal/contractworkflowengine/datatype/eventtype"
)

// Create and Renew each start a new contract instance in DRAFT. Its PDF must
// render on that event, not wait for the retry sweep.
func TestANewContractInstanceIsRenderedOnItsEvent(t *testing.T) {
	for _, typ := range []cweeventtype.EventType{cweeventtype.Create, cweeventtype.Renew} {
		if !contractLifecycleEventTypes[typ.String()] {
			t.Errorf("%s must trigger PDF regeneration", typ)
		}
	}
}
