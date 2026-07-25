package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	dcstodcs2 "digital-contracting-service/internal/dcstodcs"
	pq2 "digital-contracting-service/internal/dcstodcs/db/pg"

	didservice "digital-contracting-service/gen/did_service"

	genauth "digital-contracting-service/gen/auth"
	c2paservice "digital-contracting-service/gen/c2_pa_service"
	contractstoragearchive "digital-contracting-service/gen/contract_storage_archive"
	contractworkflowengine "digital-contracting-service/gen/contract_workflow_engine"
	dcstodcs "digital-contracting-service/gen/dcs_to_dcs"
	pdfgeneration "digital-contracting-service/gen/pdf_generation"
	processauditandcompliance "digital-contracting-service/gen/process_audit_and_compliance"
	semantichubgen "digital-contracting-service/gen/semantic_hub"
	signaturemanagement "digital-contracting-service/gen/signature_management"
	templatecatalogueintegration "digital-contracting-service/gen/template_catalogue_integration"
	templaterepository "digital-contracting-service/gen/template_repository"
	"digital-contracting-service/internal/auth"
	pg "digital-contracting-service/internal/auth/db/pq"
	oid4vprequest "digital-contracting-service/internal/auth/oid4vp/request"
	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/db/pq"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/base/federation"
	"digital-contracting-service/internal/base/hsm"
	"digital-contracting-service/internal/base/identity"
	"digital-contracting-service/internal/base/ipfs"
	"digital-contracting-service/internal/base/tsa"
	"digital-contracting-service/internal/base/validation"
	contractworkflowengine2 "digital-contracting-service/internal/contractworkflowengine"
	cwecommand "digital-contracting-service/internal/contractworkflowengine/command"
	cwerepo "digital-contracting-service/internal/contractworkflowengine/db/pg"
	"digital-contracting-service/internal/contractworkflowengine/deployevent"
	"digital-contracting-service/internal/middleware"
	pdfevent "digital-contracting-service/internal/pdfgeneration/event"
	"digital-contracting-service/internal/pdfgeneration/pdfcore"
	"digital-contracting-service/internal/pdfgeneration/provenance"
	"digital-contracting-service/internal/semantichub"
	"digital-contracting-service/internal/service"
	smrepo "digital-contracting-service/internal/signingmanagement/db/pg"
	fcclient "digital-contracting-service/internal/templatecatalogueintegration/client"
	tplrepo "digital-contracting-service/internal/templaterepository/db/pg"
	"digital-contracting-service/internal/webhookplatform"
	"digital-contracting-service/migrations"
	"digital-contracting-service/migrations/fcschemas"

	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go"
	"goa.design/clue/debug"
	"goa.design/clue/log"
)

// computeListenURL derives the HTTP listen address from the same flags/env
// var handleHTTPServer's caller used to compute inline just before binding.
// Hoisted out so a bootstrap server (see startBootstrapServer) can claim the
// same address immediately at process start, before the PKCS#11 token is
// necessarily provisioned.
func computeListenURL(ctx context.Context, hostF, domainF, httpPortF string, secureF bool) *url.URL {
	if hostF != "local" {
		log.Fatal(ctx, fmt.Errorf("invalid host argument: %q (valid hosts: local)", hostF))
	}
	address := "http://0.0.0.0:8991"
	if os.Getenv("DCS_BACKEND_PORT") != "" {
		address = fmt.Sprintf("http://0.0.0.0:%s", os.Getenv("DCS_BACKEND_PORT"))
	}
	u, err := url.Parse(address)
	if err != nil {
		log.Fatalf(ctx, err, "invalid URL %#v\n", address)
	}
	if secureF {
		u.Scheme = "https"
	}
	if domainF != "" {
		u.Host = domainF
	}
	if httpPortF != "" {
		h, _, err := net.SplitHostPort(u.Host)
		if err != nil {
			log.Fatalf(ctx, err, "invalid URL %#v\n", u.Host)
		}
		u.Host = net.JoinHostPort(h, httpPortF)
	} else if u.Port() == "" {
		u.Host = net.JoinHostPort(u.Host, "80")
	}
	return u
}

// startBootstrapServer claims the service's listen address immediately at
// process start and answers 503 (with a short explanation) to every request
// until it's shut down. Exists so the pod is Ready (per Kubernetes — no
// readinessProbe is configured, so "container running" is sufficient) the
// moment the process starts, rather than only once hsm.Open succeeds:
// hsm.Open needs the PKCS#11 token that hsm-provision-job.yaml's post-install
// hook creates, and that hook only runs once the whole release is Healthy —
// a circular wait if this pod can't report healthy until the hook has
// already run. DCS-IR-HI-01 (no software fallback for signing) is unchanged:
// this only defers *when* the real handlers start serving, not what key
// material they end up using.
func startBootstrapServer(ctx context.Context, u *url.URL) *http.Server {
	srv := &http.Server{
		Addr: u.Host,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("starting up: waiting for PKCS#11 signing key material to be provisioned\n"))
		}),
		ReadHeaderTimeout: time.Second * 60,
	}
	go func() {
		log.Printf(ctx, "bootstrap HTTP server listening on %q (waiting for HSM)", u.Host)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Errorf(ctx, err, "bootstrap HTTP server error")
		}
	}()
	return srv
}

// openHSMWithRetry retries hsm.Open until it succeeds, instead of the
// immediate log.Fatalf every other HSM/config error in main() uses. This is
// the one call in the chain that's expected to fail transiently at fresh
// install/upgrade (see startBootstrapServer's comment) — hsm-provision.sh
// creates the token and all five keys in one run, so once Open succeeds the
// downstream hsmClient.Signer(label) calls are not expected to need the same
// treatment.
func openHSMWithRetry(ctx context.Context) *hsm.HSM {
	const interval = 5 * time.Second
	attempt := 0
	for {
		attempt++
		hsmClient, err := hsm.Open(hsm.ConfigFromEnv())
		if err == nil {
			return hsmClient
		}
		if attempt == 1 || attempt%12 == 0 { // log immediately, then once/minute
			log.Printf(ctx, "waiting for PKCS#11 token (attempt %d): %v", attempt, err)
		}
		time.Sleep(interval)
	}
}

func main() {
	// Define command line flags, add any other flag required to configure the
	// service.
	var (
		hostF     = flag.String("host", "local", "Server host (valid values: local)")
		domainF   = flag.String("domain", "", "Host domain name (overrides host domain specified in service design)")
		httpPortF = flag.String("http-port", "", "HTTP port (overrides host HTTP port specified in service design)")
		secureF   = flag.Bool("secure", false, "Use secure scheme (https or grpcs)")
		dbgF      = flag.Bool("debug", false, "Log request and response bodies")
		envF      = flag.String("env", "", "Set environment file for the service")
	)
	flag.Parse()

	if envF != nil && *envF != "" {
		if err := loadDotenvFile(*envF); err != nil {
			_, err := fmt.Fprintf(os.Stderr, "startup configuration error: %v\n", err)
			if err != nil {
				return
			}
			os.Exit(1)
		}
	} else {
		if err := loadDotenvIfPresent(); err != nil {
			_, err := fmt.Fprintf(os.Stderr, "startup configuration error: %v\n", err)
			if err != nil {
				return
			}
			os.Exit(1)
		}
	}

	// Setup logger. Replace logger with your own log package of choice.
	format := log.FormatJSON
	if log.IsTerminal() {
		format = log.FormatTerminal
	}
	ctx := log.Context(context.Background(), log.WithFormat(format))
	if *dbgF {
		ctx = log.Context(ctx, log.WithDebug())
		log.Debugf(ctx, "debug logs enabled")
	}
	log.Print(ctx, log.KV{K: "http-port", V: *httpPortF})

	db, err := NewDatabaseConnection()
	if err != nil {
		log.Fatalf(ctx, err, "Could not connect to database")
	}
	defer func(db *sqlx.DB) {
		err := db.Close()
		if err != nil {
			fmt.Printf("could not close database connection: %v\n", err)
		}
	}(db)

	log.Printf(ctx, "Connecting to database")

	// Run database migrations
	if err := migrations.Run(db); err != nil {
		log.Fatalf(ctx, err, "Could not run database migrations")
		os.Exit(1)
	}

	// DCS_PUBLIC_URL is the base of every absolute IRI a produced document
	// carries (@context, sh:shapesGraph, dcterms:conformsTo anchors, C2PA
	// remote manifests) — these must dereference for external consumers.
	if strings.TrimSpace(os.Getenv("DCS_PUBLIC_URL")) == "" {
		log.Fatalf(ctx, errors.New("dcs configuration missing"), "DCS_PUBLIC_URL must be set: produced documents carry absolute, resolvable IRIs based on it")
	}

	// Seed the Semantic Hub genesis schemas (JSON-LD context, SHACL shapes,
	// validation profile) and anchor document production to the active
	// versions. The SemanticHub service re-runs the anchor refresh after
	// every activation/rollback.
	if err := semantichub.Seed(ctx, db); err != nil {
		log.Fatalf(ctx, err, "Could not seed the Semantic Hub genesis schemas")
	}
	if err := service.RefreshValidationAnchors(ctx, db); err != nil {
		log.Fatalf(ctx, err, "Could not anchor validation to the Semantic Hub's active schemas")
	}

	validation.SetShapeSource(semantichub.HubShapeSource{DB: db})

	// Open the PKCS#11 token that holds every private key (DCS-IR-HI-01) — no
	// software fallback once open, but opening itself is retried rather than
	// immediately fatal: a fresh install/upgrade's hsm-provision Job may not
	// have run yet (see openHSMWithRetry's doc comment). The bootstrap server
	// claims the listen address now so the pod can report healthy while this
	// retries; handleHTTPServer takes over the same address once ready.
	listenURL := computeListenURL(ctx, *hostF, *domainF, *httpPortF, *secureF)
	bootstrapSrv := startBootstrapServer(ctx, listenURL)
	hsmClient := openHSMWithRetry(ctx)
	defer func() {
		if err := hsmClient.Close(); err != nil {
			log.Errorf(ctx, err, "Could not close PKCS#11 token")
		}
	}()

	didFilePath := os.Getenv("DCS_DID")

	didSigner, err := hsmClient.Signer(hsm.KeyLabelDID())
	if err != nil {
		log.Fatalf(ctx, err, "Could not load HSM DID signing key")
	}

	log.Printf(ctx, "Reading did.json")
	didDocument, err := identity.NewDIDDocument(didFilePath, didSigner)
	if err != nil {
		log.Fatalf(ctx, err, "Could not read did document")
	}

	var euTrustPool *identity.EUTrustPool
	if base.GetEnvOrDefault("DCS_FORCE_EIDAS_CERT", false) {
		log.Printf(ctx, "Start building EU trust pool")
		trustPool := identity.NewEUTrustPool()
		if err := trustPool.Refresh(ctx); err != nil {
			log.Fatalf(ctx, err, "Building EU trust pool")
		}
		count, _, errs := trustPool.Stats()
		log.Printf(ctx, "EU trust pool ready: %d certificates (%d lists skipped)", count, len(errs))

		// Keep it fresh in the background.
		go trustPool.StartAutoRefresh(ctx, identity.DefaultRefreshInterval)

		euTrustPool = trustPool
	}

	err = didDocument.VerifyEIDASCertificate(euTrustPool)
	if err != nil {
		log.Fatalf(ctx, err, "Could not verify certificate")
	}

	// Initialize OIDC validator and JWT authenticator.
	authCfg, err := loadAuthConfig(ctx)
	if err != nil {
		log.Fatalf(ctx, err, "Could not load auth config")
	}

	// Sign OpenID4VP authorization request objects (JAR) with the HSM key; the
	// public JWK is embedded in the JWT header and the key label is its kid.
	jarLabel := hsm.KeyLabelJAR()
	jarSigner, err := hsmClient.Signer(jarLabel)
	if err != nil {
		log.Fatalf(ctx, err, "Could not load HSM JAR signing key")
	}
	jarJWK, err := hsmClient.PublicJWK(jarLabel)
	if err != nil {
		log.Fatalf(ctx, err, "Could not read HSM JAR public key")
	}
	requestSigner, err := oid4vprequest.NewHSMSigner(jarLabel, jarSigner, jarJWK, hsm.SignES256)
	if err != nil {
		log.Fatalf(ctx, err, "Could not build OID4VP request signer")
	}
	authCfg.RequestSigner = requestSigner

	// The Document-Retrieval signing ceremony's request object declares
	// client_id_scheme=x509_san_dns (docretrieval.go) — a real wallet resolves
	// trust from the leaf certificate's SAN, not a bare jwk, so it is signed
	// with the DCS's own DID/hostname certificate chain instead of the HSM JAR
	// key above (already verified, just above, to carry a SAN matching its
	// hostname).
	docRetrievalSigner, err := oid4vprequest.NewX5CSigner(didDocument)
	if err != nil {
		log.Fatalf(ctx, err, "Could not build OID4VP document-retrieval request signer")
	}
	docRetrievalClientID, err := docRetrievalSigner.ClientID()
	if err != nil {
		log.Fatalf(ctx, err, "Could not resolve document-retrieval client_id")
	}
	systemClients, err := loadSystemClients()
	if err != nil {
		log.Fatalf(ctx, err, "Invalid system client configuration")
	}
	hydraJWTValidator, err := middleware.NewHydraJWTValidator(ctx, middleware.HydraJWTConfig{
		PublicIssuerURL:   authCfg.Hydra.PublicIssuerURL(),
		InternalIssuerURL: authCfg.Hydra.InternalIssuerURL(),
		ClientID:          authCfg.Hydra.ClientID(),
		SystemClients:     systemClients,
	})
	if err != nil {
		log.Fatalf(ctx, err, "Failed to initialize Hydra JWT validator")
	}

	// Initialize IPFS client
	ipfsTenantBaseURL := os.Getenv("IPFS_TENANT_BASE_URL")
	mfsBaseURL := os.Getenv("IPFS_MFS_BASE_URL")
	if ipfsTenantBaseURL == "" || mfsBaseURL == "" {
		log.Fatalf(ctx, nil, "IPFS configuration missing: IPFS_TENANT_BASE_URL and IPFS_MFS_BASE_URL environment variables must be specified")
	}
	ipfsAPIClient := ipfs.NewClient(ipfsTenantBaseURL, mfsBaseURL)

	aAttemptRepo := &pg.PostgresAccessAttemptRepo{}
	lockRepo := &pg.PostgresIPLockoutRepo{}
	jwtAuth := auth.NewJWTAuthenticator(hydraJWTValidator, db, aAttemptRepo, lockRepo)

	ctRepo := tplrepo.PostgresContractTemplateRepo{}
	ctRTRepo := tplrepo.PostgresReviewTaskRepo{}
	ctATRepo := tplrepo.PostgresApprovalTaskRepo{}

	cweRepo := cwerepo.PostgresContractRepo{}
	cweRTRepo := cwerepo.PostgresReviewTaskRepo{}
	cweATRepo := cwerepo.PostgresApprovalTaskRepo{}
	cweNTRepo := cwerepo.PostgresNegotiationTaskRepo{}
	cweNRepo := cwerepo.PostgresNegotiationRepo{}
	cweCTRepo := cwerepo.PostgresContractTemplateRepo{}
	cweCronJob := contractworkflowengine2.CronJob{DB: db, CRepo: &cweRepo}
	cweCronJob.Start(ctx, db)

	aRepo := pq.PostgresAuditTrailRepository{}

	tsaURL := os.Getenv("TSA_URL")
	if tsaURL == "" {
		log.Fatalf(ctx, nil, "TSA_URL is not set")
	}
	tsaClient, err := tsa.NewClient(tsaURL)
	if err != nil {
		log.Fatalf(ctx, err, "failed to initialize TSA client")
	}

	// Connect to NATS (use NATS_URL env var or default)
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	cepPubClient, err := event.NewNatsPubClient(conf.EventBusTopic(), natsURL)
	if err != nil {
		log.Fatalf(ctx, err, "Could not connect to events bus")
	}
	defer func(client *event.CloudEventPubClient) {
		err := client.Close()
		if err != nil {
			log.Errorf(ctx, err, "Could not close cloud event bus client")
		}
	}(cepPubClient)

	did, err := didDocument.GetID()
	if err != nil {
		log.Fatalf(ctx, err, "could not read DID")
	}

	outboxProcessor := event.OutboxProcessor{
		DB:           db,
		CEPPubClient: cepPubClient,
		IPFSClient:   ipfsAPIClient,
		ARepo:        &aRepo,
		TSAClient:    tsaClient,
	}
	err = outboxProcessor.Start(ctx)
	if err != nil {
		log.Fatalf(ctx, err, "failed to start outbox processor")
	}

	cepSubClient, err := event.NewNatsSubClient(conf.EventBusTopic(), natsURL)
	if err != nil {
		log.Fatalf(ctx, err, "Could not connect to events bus")
	}
	defer func(client *event.CloudEventSubClient) {
		err := client.Close()
		if err != nil {
			log.Errorf(ctx, err, "Could not close cloud event bus client")
		}
	}(cepSubClient)

	syncRepo := pq2.PostgresSyncRepository{}
	// Federation trust gate (ADR-19): agreement credential verification + the
	// local policy endpoint, consulted on both the outbound (shipContractPDF,
	// here) and inbound (PostPdf, service.NewDcsToDcs below) paths.
	trustGate := dcstodcs2.TrustGate{PDPURL: os.Getenv("DCS_TRUST_PDP_URL")}
	dcsToDcsSynchronizer := dcstodcs2.DCSToDCSSynchronizer{
		DB:          db,
		CRepo:       &cweRepo,
		SRepo:       &syncRepo,
		IPFSClient:  ipfsAPIClient,
		DIDDocument: *didDocument,
		TrustGate:   trustGate,
	}
	dcsToDcsSynchronizer.StartSynchronizerJob(ctx, cepSubClient)

	if os.Getenv("DCS_DEBUG_EVENTING") == "true" {
		event.StartEventLogger(ctx, cepSubClient)
	}

	auditTrailReader := base.AuditTrailReader{
		IPFSClient: ipfsAPIClient,
		ARepo:      &aRepo,
	}

	archiveNotaryURL := strings.TrimSpace(os.Getenv("ORCE_ARCHIVE_NOTARY_URL"))
	var archiveNotaryClient cwecommand.ArchiveNotary
	if archiveNotaryURL != "" {
		archiveNotaryClient = cwecommand.NewHTTPArchiveNotaryClient(archiveNotaryURL, os.Getenv("ORCE_ARCHIVE_AUDIT_LOG_BEARER_TOKEN"))
	}

	// Contract deployment (UC-05-01): the Contract Target
	// System client is optional — without CONTRACT_TARGET_URL set, deploy
	// dispatches are still recorded (correlation ID, content hash, archive
	// evidence) but no outbound call is made; the target's own callback
	// (POST /contract/deployment/callback) is the authoritative signal.
	cweDeploymentRepo := &cwerepo.PostgresDeploymentRepo{}
	var contractTargetClient cwecommand.ContractTargetClient
	if targetURL := cwecommand.ContractTargetURL(); targetURL != "" {
		contractTargetClient = cwecommand.NewHTTPContractTargetClient(targetURL)
	}

	// Initialize the Federated Catalogue client.
	fcURL := os.Getenv("FEDERATED_CATALOGUE_API_URL")
	fcClientID := os.Getenv("FEDERATED_CATALOGUE_CLIENT_ID")
	fcClientSecret := os.Getenv("FEDERATED_CATALOGUE_CLIENT_SECRET")
	var templateCatalogueClient *fcclient.FederatedCatalogueClient
	if fcURL != "" && fcClientID != "" && fcClientSecret != "" {
		fcRealmURL := strings.TrimSpace(os.Getenv("FC_KEYCLOAK_REALM_URL"))
		if fcRealmURL == "" {
			log.Fatalf(ctx, nil, "Federated Catalogue requires FC_KEYCLOAK_REALM_URL (Keycloak for FC only, not Hydra)")
		}
		templateCatalogueClient, err = fcclient.NewFederatedCatalogueClient(fcclient.Config{
			APIURL:           fcURL,
			KeycloakRealmURL: fcRealmURL,
			ClientID:         fcClientID,
			ClientSecret:     fcClientSecret,
		})
		if err != nil {
			log.Fatalf(ctx, err, "failed to initialize Federated Catalogue client")
		}
		if err := fcschemas.SyncWithRetry(ctx, templateCatalogueClient); err != nil {
			log.Fatalf(ctx, err, "failed to sync federated catalogue schemas")
		}
	}

	// Initialize the webhook platform (ORCE integration).
	webhookStore := webhookplatform.NewSubscriptionStore()
	webhookDispatcher := webhookplatform.NewDispatcher(webhookStore)
	webhookPlatform := webhookplatform.New(
		webhookStore,
		webhookDispatcher,
		func(ctx context.Context, token string) (string, error) {
			info, err := hydraJWTValidator.ValidateToken(ctx, token)
			if err != nil {
				return "", err
			}
			return info.ParticipantDID, nil
		},
		nil,
	)

	// Start the NATS→Webhook bridge: automatically fans out to all registered
	// webhook subscribers whenever a DCS lifecycle event fires on the event bus.
	webhookSubClient, err := event.NewNatsSubClient(conf.EventBusTopic(), natsURL)
	if err != nil {
		log.Fatalf(ctx, err, "Could not create webhook NATS subscriber")
	}
	defer func(webhookSubClient *event.CloudEventSubClient) {
		err := webhookSubClient.Close()
		if err != nil {
			log.Errorf(ctx, err, "failed to close webhook subscriber")
		}
	}(webhookSubClient)
	go func() {
		if err := webhookplatform.StartNATSBridge(webhookSubClient, webhookDispatcher); err != nil {
			log.Fatalf(ctx, err, "Could not start webhook NATS bridge")
		}
	}()

	// Sign contract-lifecycle VCs (DCS-OR-C2PA-004) with the HSM VC key,
	// producing an ecdsa-rdfc-2019 Data Integrity proof.
	issuerDID := os.Getenv("ISSUER_DID")
	vcKeyLabel := hsm.KeyLabelVC()
	vcHSMSigner, err := hsmClient.Signer(vcKeyLabel)
	if err != nil {
		log.Fatalf(ctx, err, "Could not load HSM VC signing key")
	}
	vcSigner := provenance.NewHSMVCSigner(vcHSMSigner, vcKeyLabel)

	// Sign COSE Sig_structure bytes for pdf-core's C2PA manifests with the HSM
	// C2PA key. pdf-core prepares the Sig_structures; the DCS signs them in-process
	// via the pdf-core client and posts them back for embedding (pdf-core is keyless).
	c2paSigner, err := hsmClient.Signer(hsm.KeyLabelC2PA())
	if err != nil {
		log.Fatalf(ctx, err, "Could not load HSM C2PA signing key")
	}

	// Initialize OCM-W Status List Service client (DCS-OR-C2PA-005).
	statusListServiceURL := os.Getenv("STATUSLIST_SERVICE_URL")
	if statusListServiceURL == "" {
		log.Fatalf(ctx, nil, "STATUSLIST_SERVICE_URL is required (DCS-OR-C2PA-005)")
	}
	if err := probeHTTPUntilReady(3*time.Minute, func() error {
		return probeHTTPAny(statusListServiceURL+"/health", statusListServiceURL+"/v1/metrics/health")
	}); err != nil {
		log.Fatalf(ctx, err, "status list service not reachable at %s", statusListServiceURL)
	}
	statusListTenantID := os.Getenv("STATUSLIST_TENANT_ID") // defaults to "default" when empty
	statusListPublisher := provenance.NewOCMWStatusListPublisher(statusListServiceURL, issuerDID, statusListTenantID)

	// Initialize pdf-core client (PDF rendering + C2PA provenance microservice).
	pdfCoreURL := os.Getenv("PDF_CORE_URL")
	if pdfCoreURL == "" {
		log.Fatalf(ctx, nil, "PDF_CORE_URL is required")
	}
	if err := probeHTTPUntilReady(3*time.Minute, func() error {
		return probeHTTP(pdfCoreURL + "/version")
	}); err != nil {
		log.Fatalf(ctx, err, "pdf-core not reachable at %s", pdfCoreURL)
	}
	pdfCoreClient := pdfcore.New(pdfCoreURL, func(sigStructure []byte) ([]byte, error) {
		return hsm.SignES256(c2paSigner, sigStructure)
	})

	smCRepo := smrepo.PostgresContractRepo{
		IPFSClient: ipfsAPIClient,
		PDFCore:    pdfCoreClient,
	}

	// Build and sign this instance's federation agreement credential once at
	// startup (ADR-19): issuer = this instance's own DID, termsOfUse names the
	// embedded federation rules document by its public policy URL and hash.
	rulesPolicyURL := federation.RulesPolicyURL(os.Getenv("DCS_PUBLIC_URL"))
	agreementCredential, err := federation.BuildAgreementCredential(ctx, vcSigner, issuerDID, rulesPolicyURL)
	if err != nil {
		log.Fatalf(ctx, err, "failed to build federation agreement credential")
	}

	didService, err := service.NewDIDService(*didDocument, agreementCredential, federation.Rules())
	if err != nil {
		log.Fatalf(ctx, err, "failed to create did service")
	}

	// Initialize the service.
	var (
		authSvc                         genauth.Service
		contractStorageArchiveSvc       contractstoragearchive.Service
		contractWorkflowEngineSvc       contractworkflowengine.Service
		dcsToDcsSvc                     dcstodcs.Service
		pdfGenerationSvc                pdfgeneration.Service
		processAuditAndComplianceSvc    processauditandcompliance.Service
		signatureManagementSvc          signaturemanagement.Service
		templateCatalogueIntegrationSvc templatecatalogueintegration.Service
		templateRepositorySvc           templaterepository.Service
		didSrv                          didservice.Service
		c2paSvc                         c2paservice.Service
		semanticHubSvc                  semantichubgen.Service
	)
	{
		presentationRepo := pg.NewPostgresPresentationAttemptRepo(db)
		authSvc, err = service.NewAuth(db, presentationRepo, authCfg)
		if err != nil {
			log.Fatalf(ctx, err, "auth service init failed")
		}

		contractStorageArchiveSvc = service.NewContractStorageArchive(db, jwtAuth, &cweRepo, *didDocument, auditTrailReader)
		contractWorkflowEngineSvc = service.NewContractWorkflowEngine(db, jwtAuth, &cweRepo, &cweRTRepo, &cweATRepo, &cweNTRepo, &cweNRepo, &cweCTRepo, &syncRepo, euTrustPool, templateCatalogueClient, auditTrailReader, *didDocument, ipfsAPIClient, archiveNotaryClient, tsaClient, cweDeploymentRepo, contractTargetClient)
		dcsToDcsSvc = service.NewDcsToDcs(db, jwtAuth, &cweRepo, &cweRTRepo, &cweATRepo, &cweNTRepo, &cweNRepo, &cweCTRepo, &syncRepo, euTrustPool, *didDocument, ipfsAPIClient, pdfCoreClient, trustGate)
		pdfGenerationSvc = service.NewPDFGeneration(db, jwtAuth, ipfsAPIClient, &cweRepo, &ctRepo, &smCRepo, pdfCoreClient, issuerDID, provenance.NewLocalVCIssuer(vcSigner, issuerDID, statusListPublisher), did)
		c2paSvc = service.NewC2PAService(db, ipfsAPIClient, &cweRepo, pdfCoreClient, issuerDID, provenance.NewLocalVCIssuer(vcSigner, issuerDID, statusListPublisher))
		processAuditAndComplianceSvc = service.NewProcessAuditAndCompliance(db, jwtAuth, auditTrailReader, &ctRepo, &cweRepo, &cweATRepo)
		signatureManagementSvc = service.NewSignatureManagement(db, jwtAuth, &smCRepo, &smrepo.PostgresCeremonyRepo{}, auditTrailReader, vcSigner, issuerDID, ipfsAPIClient, pdfCoreClient, &cweRepo, archiveNotaryClient, tsaClient, provenance.NewLocalVCIssuer(vcSigner, issuerDID, statusListPublisher), requestSigner, authCfg.Hydra.ClientID(), authCfg.PublicAPIBase, docRetrievalSigner, docRetrievalClientID, authCfg.PIDDCQLQuery, authCfg.DCQLQuery, authCfg.Trust)
		templateCatalogueIntegrationSvc = service.NewTemplateCatalogueIntegration(db, jwtAuth, templateCatalogueClient)
		templateRepositorySvc = service.NewTemplateRepository(db, jwtAuth, &ctRepo, &ctRTRepo, &ctATRepo, templateCatalogueClient, auditTrailReader, vcSigner, issuerDID)
		didSrv = didService
		semanticHubSvc = service.NewSemanticHub(db, jwtAuth)
	}

	// Channel used by background workers and signal handler to notify main to exit.
	errc := make(chan error)

	// Start the PDF lifecycle C2PA subscriber (appends C2PA assertions on state changes).
	// Only start when a real signing URL is configured; without one, the subscriber
	// would attempt signing on every CWE event and log spurious HTTP errors.
	pdfSubClient, err := event.NewNatsSubClient(conf.EventBusTopic(), natsURL)
	if err != nil {
		log.Fatalf(ctx, err, "Could not create PDF generation NATS subscriber")
	}
	defer func(pdfSubClient *event.CloudEventSubClient) {
		err := pdfSubClient.Close()
		if err != nil {
			log.Errorf(ctx, err, "Could not close PDF subscriber")
		}
	}(pdfSubClient)
	pdfSub := &pdfevent.Subscriber{
		DB:         db,
		IPFSClient: ipfsAPIClient,
		CRepo:      &cweRepo,
		TRepo:      &ctRepo,
		PDFCore:    pdfCoreClient,
		IssuerDID:  issuerDID,
		LocalPeer:  did,
		VCIssuer:   provenance.NewLocalVCIssuer(vcSigner, issuerDID, statusListPublisher),
	}
	go func() {
		if err := pdfSub.Start(pdfSubClient); err != nil {
			errc <- fmt.Errorf("could not start PDF generation subscriber: %w", err)
		}
	}()

	// Start the auto-deploy subscriber (DCS-FR-CWE-06): once the signing
	// workflow completes (APPLIED_SIGNATURE), it calls the same
	// cwecommand.Deployer the manual POST /contract/deploy endpoint uses.
	deploySubClient, err := event.NewNatsSubClient(conf.EventBusTopic(), natsURL)
	if err != nil {
		log.Fatalf(ctx, err, "Could not create contract-deployment NATS subscriber")
	}
	defer func(deploySubClient *event.CloudEventSubClient) {
		err := deploySubClient.Close()
		if err != nil {
			log.Errorf(ctx, err, "Could not close contract-deployment subscriber")
		}
	}(deploySubClient)
	deploySub := &deployevent.Subscriber{
		Deployer: &cwecommand.Deployer{
			DB:             db,
			CRepo:          &cweRepo,
			DeploymentRepo: cweDeploymentRepo,
			Target:         contractTargetClient,
		},
	}
	go func() {
		if err := deploySub.Start(deploySubClient); err != nil {
			errc <- fmt.Errorf("could not start contract-deployment subscriber: %w", err)
		}
	}()

	// Wrap the service in endpoints that can be invoked from other service
	// potentially running in different processes.
	var (
		authEndpoints                         *genauth.Endpoints
		contractStorageArchiveEndpoints       *contractstoragearchive.Endpoints
		contractWorkflowEngineEndpoints       *contractworkflowengine.Endpoints
		dcsToDcsEndpoints                     *dcstodcs.Endpoints
		pdfGenerationEndpoints                *pdfgeneration.Endpoints
		processAuditAndComplianceEndpoints    *processauditandcompliance.Endpoints
		signatureManagementEndpoints          *signaturemanagement.Endpoints
		templateCatalogueIntegrationEndpoints *templatecatalogueintegration.Endpoints
		templateRepositoryEndpoints           *templaterepository.Endpoints
		didEntpoints                          *didservice.Endpoints
		c2paEndpoints                         *c2paservice.Endpoints
		semanticHubEndpoints                  *semantichubgen.Endpoints
	)
	{
		authEndpoints = genauth.NewEndpoints(authSvc)
		authEndpoints.Use(debug.LogPayloads())
		authEndpoints.Use(log.Endpoint)
		contractStorageArchiveEndpoints = contractstoragearchive.NewEndpoints(contractStorageArchiveSvc)
		contractStorageArchiveEndpoints.Use(debug.LogPayloads())
		contractStorageArchiveEndpoints.Use(log.Endpoint)
		contractWorkflowEngineEndpoints = contractworkflowengine.NewEndpoints(contractWorkflowEngineSvc)
		contractWorkflowEngineEndpoints.Use(debug.LogPayloads())
		contractWorkflowEngineEndpoints.Use(log.Endpoint)
		dcsToDcsEndpoints = dcstodcs.NewEndpoints(dcsToDcsSvc)
		dcsToDcsEndpoints.Use(debug.LogPayloads())
		dcsToDcsEndpoints.Use(log.Endpoint)
		pdfGenerationEndpoints = pdfgeneration.NewEndpoints(pdfGenerationSvc)
		pdfGenerationEndpoints.Use(debug.LogPayloads())
		pdfGenerationEndpoints.Use(log.Endpoint)
		processAuditAndComplianceEndpoints = processauditandcompliance.NewEndpoints(processAuditAndComplianceSvc)
		processAuditAndComplianceEndpoints.Use(auth.AccessMetadataMiddleware)
		processAuditAndComplianceEndpoints.Use(debug.LogPayloads())
		processAuditAndComplianceEndpoints.Use(log.Endpoint)
		signatureManagementEndpoints = signaturemanagement.NewEndpoints(signatureManagementSvc)
		signatureManagementEndpoints.Use(debug.LogPayloads())
		signatureManagementEndpoints.Use(log.Endpoint)
		templateCatalogueIntegrationEndpoints = templatecatalogueintegration.NewEndpoints(templateCatalogueIntegrationSvc)
		templateCatalogueIntegrationEndpoints.Use(debug.LogPayloads())
		templateCatalogueIntegrationEndpoints.Use(log.Endpoint)
		templateRepositoryEndpoints = templaterepository.NewEndpoints(templateRepositorySvc)
		templateRepositoryEndpoints.Use(debug.LogPayloads())
		templateRepositoryEndpoints.Use(log.Endpoint)
		didEntpoints = didservice.NewEndpoints(didSrv)
		didEntpoints.Use(debug.LogPayloads())
		didEntpoints.Use(log.Endpoint)
		c2paEndpoints = c2paservice.NewEndpoints(c2paSvc)
		c2paEndpoints.Use(debug.LogPayloads())
		c2paEndpoints.Use(log.Endpoint)
		semanticHubEndpoints = semantichubgen.NewEndpoints(semanticHubSvc)
		semanticHubEndpoints.Use(debug.LogPayloads())
		semanticHubEndpoints.Use(log.Endpoint)
	}

	// Setup interrupt handler. This optional step configures the process so
	// that SIGINT and SIGTERM signals cause the service to stop gracefully.
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)

	// Hand off from the bootstrap server (up since before hsm.Open) to the
	// real one on the same address — safe to do serially since Shutdown
	// blocks until the listener's actually released.
	if err := bootstrapSrv.Shutdown(ctx); err != nil {
		log.Errorf(ctx, err, "failed to shut down bootstrap HTTP server")
	}
	handleHTTPServer(ctx, listenURL, authEndpoints, contractStorageArchiveEndpoints, contractWorkflowEngineEndpoints, dcsToDcsEndpoints, pdfGenerationEndpoints, processAuditAndComplianceEndpoints, signatureManagementEndpoints, templateCatalogueIntegrationEndpoints, templateRepositoryEndpoints, didEntpoints, c2paEndpoints, semanticHubEndpoints, webhookPlatform, &wg, errc, *dbgF)

	// Wait for signal.
	log.Printf(ctx, "exiting (%v)", <-errc)

	// Send cancellation signal to the goroutines.
	cancel()

	wg.Wait()
	log.Printf(ctx, "exited")
}
