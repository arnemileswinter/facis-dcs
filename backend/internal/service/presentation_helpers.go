package service

import (
	genauth "digital-contracting-service/gen/auth"
	"strings"
)

// PresentationConfig exposes the auth service's CV/tenant/UI base path
// configuration so the HTTP-layer presentation proxy can build forwarding
// targets and completion redirect URLs without re-reading environment
// variables.
//
// Returns (cvURL, tenantNamespace, uiBasePath). All values may be empty
// when the auth service is not the concrete *authSvc, in which case the
// caller is expected to fall back to defaults or refuse the request.
func PresentationConfig(svc genauth.Service) (cvURL, tenant, uiBasePath string) {
	s, ok := svc.(*authSvc)
	if !ok {
		return "", "", ""
	}
	return s.cvURL, s.presentationNamespace, s.uiBasePath
}

// MarkPresentationCompleted flips the in-memory presentation state for the
// given request id to completed and stores the redirect location used by
// the frontend's polling loop. Returns true when the state was found and
// updated, false when no presentation with that id is being tracked.
func MarkPresentationCompleted(svc genauth.Service, requestID, location string) bool {
	s, ok := svc.(*authSvc)
	if !ok {
		return false
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return false
	}

	s.presentationMu.Lock()
	defer s.presentationMu.Unlock()
	stored := s.presentationState[requestID]
	if stored == nil {
		return false
	}
	stored.Completed = true
	if location != "" {
		stored.Location = location
	}
	return true
}
