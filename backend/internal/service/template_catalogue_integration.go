package service

import (
	"context"
	templatecatalogueintegration "digital-contracting-service/gen/template_catalogue_integration"
	"digital-contracting-service/internal/auth"

	"goa.design/clue/log"
)

type templateCatalogueIntegrationsrvc struct {
	auth.JWTAuthenticator
}

func NewTemplateCatalogueIntegration(jwtAuth auth.JWTAuthenticator) templatecatalogueintegration.Service {
	return &templateCatalogueIntegrationsrvc{JWTAuthenticator: jwtAuth}
}

func (s *templateCatalogueIntegrationsrvc) Discover(ctx context.Context, p *templatecatalogueintegration.DiscoverPayload) (res any, err error) {
	log.Printf(ctx, "templateCatalogueIntegration.discover")
	return
}

func (s *templateCatalogueIntegrationsrvc) Request(ctx context.Context, p *templatecatalogueintegration.RequestPayload) (res any, err error) {
	log.Printf(ctx, "templateCatalogueIntegration.request")
	return
}

func (s *templateCatalogueIntegrationsrvc) Register(ctx context.Context, p *templatecatalogueintegration.RegisterPayload) (res any, err error) {
	log.Printf(ctx, "templateCatalogueIntegration.register")
	return
}
