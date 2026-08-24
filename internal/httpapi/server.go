package httpapi

import (
	"context"
	"net/http"
	"sync"
	"time"

	"cpanel/internal/auth"
	"cpanel/internal/config"
	"cpanel/internal/iplocation"
	"cpanel/internal/securebox"
	"cpanel/internal/store"
	webui "cpanel/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	sessionCookieName       = "ca_hh"
	portalSessionCookieName = "ca_edu"
)

type Server struct {
	cfg               config.Config
	store             *store.Store
	dummyPasswordHash string
	secureBox         *securebox.Box
	agentRelease      *agentReleaseResolver
	agentV2           *agentV2Service
	telegramBot       TelegramBotService
	loginRateMu       sync.Mutex
	loginAttempts     map[string]loginAttempt
	ipLocation        ipLocationResolver
	ipLocationRateMu  sync.Mutex
	ipLocationLookups map[string]time.Time
}

type TelegramBotService interface {
	SendTest(context.Context) error
}

func New(cfg config.Config, dataStore *store.Store, box *securebox.Box, telegramBots ...TelegramBotService) *Server {
	dummy, _ := auth.HashPassword("this password is intentionally never valid")
	var telegramBot TelegramBotService
	if len(telegramBots) > 0 {
		telegramBot = telegramBots[0]
	}
	return &Server{
		cfg: cfg, store: dataStore, dummyPasswordHash: dummy, secureBox: box,
		agentRelease: newAgentReleaseResolver(nil, ""), agentV2: newAgentV2Service(dataStore, box),
		telegramBot: telegramBot, loginAttempts: make(map[string]loginAttempt),
		ipLocation: iplocation.New(nil, ""), ipLocationLookups: make(map[string]time.Time),
	}
}

func (s *Server) Handler() http.Handler {
	router := chi.NewRouter()
	router.Use(recoveryMiddleware)
	router.Use(s.commonMiddleware)
	router.Use(middleware.Compress(5))

	router.Get("/ca/jk/ch", localHealth(s.handleLive))
	router.Get("/ca/jk/jx", localHealth(s.handleReady))
	router.Get("/ca/wj/{artifact}", s.handleAgentArtifact)
	router.Post("/ca/ht/hh", s.handleLogin)
	router.Get("/ca/x/{token}", s.handleSubscription)
	router.Post("/ca/edu/dl", s.handlePortalLogin)
	router.Post("/ca/edu/sq", s.handlePortalGrantRedemption)
	router.Route("/ca/edu", func(portal chi.Router) {
		portal.Use(s.requirePortal)
		portal.Get("/hh", s.handlePortalCurrentSession)
		portal.Get("/zl", s.handlePortalDashboard)
		portal.Patch("/mm", s.handlePortalPassword)
		portal.Post("/dy", s.handlePortalSubscriptionRotation)
		portal.Post("/tc", s.handlePortalLogout)
	})

	router.Route("/ca/ht", func(ops chi.Router) {
		ops.Use(s.requireAdmin)
		ops.Get("/hh", s.handleCurrentSession)
		ops.Delete("/hh/dq", s.handleLogout)
		ops.Patch("/zh/zl", s.handleUpdateAdminProfile)
		ops.Patch("/zh/mm", s.handleUpdateAdminPassword)
		ops.Post("/gj/xsmy", s.handleGenerateRealityCredentials)
		ops.Get("/zl", s.handleOverview)
		ops.Get("/ls", s.handleHistoricalData)
		ops.Get("/fwq", s.handleListMachines)
		ops.Post("/fwq", s.handleCreateMachine)
		ops.Patch("/fwq/{id}", s.handleUpdateMachine)
		ops.Delete("/fwq/{id}", s.handleDeleteMachine)
		ops.Get("/fwq/{id}/az", s.handleMachineInstallation)
		ops.Post("/fwq/{id}/sj", s.handleMachineAgentUpgrade)
		ops.Get("/fwq/{id}/sj", s.handleMachineAgentUpgradeStatus)
		ops.Delete("/fwq/{id}/sf", s.handleResetMachineAgentIdentity)
		ops.Post("/fwq/{id}/pz", s.handleCreateMachineCredential)
		ops.Get("/jd", s.handleListNodes)
		ops.Post("/jd", s.handleCreateNode)
		ops.Get("/jd/{id}", s.handleGetNode)
		ops.Patch("/jd/{id}", s.handleUpdateNode)
		ops.Delete("/jd/{id}", s.handleDeleteNode)
		ops.Post("/jd/{id}/fb", s.handlePublishNode)
		ops.Get("/qxz", s.handleListAccessGroups)
		ops.Post("/qxz", s.handleCreateAccessGroup)
		ops.Patch("/qxz/{id}", s.handleUpdateAccessGroup)
		ops.Delete("/qxz/{id}", s.handleDeleteAccessGroup)
		ops.Get("/tc", s.handleListPlans)
		ops.Post("/tc", s.handleCreatePlan)
		ops.Patch("/tc/{id}", s.handleUpdatePlan)
		ops.Delete("/tc/{id}", s.handleDeletePlan)
		ops.Get("/yh", s.handleListUsers)
		ops.Get("/yh/fwjl", s.handleListUserAccessIPs)
		ops.Post("/yh/fwjl/gs", s.handleResolveUserAccessIPLocation)
		ops.Post("/yh", s.handleCreateUser)
		ops.Post("/yh/ks/{role}", s.handleQuickCreateUser)
		ops.Get("/yh/{id}/dy", s.handleGetUserSubscription)
		ops.Post("/yh/{id}/edu", s.handleCreatePortalGrant)
		ops.Patch("/yh/{id}", s.handleUpdateUser)
		ops.Delete("/yh/{id}", s.handleDeleteUser)
		ops.Get("/ly", s.handleListRoutePolicies)
		ops.Post("/ly", s.handleCreateRoutePolicy)
		ops.Patch("/ly/{id}", s.handleUpdateRoutePolicy)
		ops.Delete("/ly/{id}", s.handleDeleteRoutePolicy)
		ops.Get("/ck", s.handleListOutbounds)
		ops.Post("/ck", s.handleCreateOutbound)
		ops.Patch("/ck/{id}", s.handleUpdateOutbound)
		ops.Delete("/ck/{id}", s.handleDeleteOutbound)
		ops.Get("/sz/dy/jl", s.handleListSubscriptionAccess)
		ops.Post("/sz/tg/cs", s.handleTestTelegramBot)
		ops.Get("/sz/{section}", s.handleListSettings)
		ops.Patch("/sz/{section}", s.handleUpdateSettings)
		ops.Get("/sj", s.handleListAuditEvents)
	})

	router.Handle("/api/*", http.NotFoundHandler())
	router.Handle("/health/*", http.NotFoundHandler())

	router.Route("/ca/cc", func(control chi.Router) {
		control.Post("/ws", s.handleAgentHandshakeEntry)
		control.Get("/fwq/jd", s.legacyAgent(http.HandlerFunc(s.handleAgentNodes)).ServeHTTP)
		control.Post("/fwq/jd", s.requireAgentV2(http.HandlerFunc(s.handleAgentV2Nodes)).ServeHTTP)
		control.Get("/jd/{jdbh}/pz", s.legacyAgent(http.HandlerFunc(s.handleAgentNodeSpec)).ServeHTTP)
		control.Post("/jd/{jdbh}/pz", s.requireAgentV2(http.HandlerFunc(s.handleAgentV2NodeSpec)).ServeHTTP)
		control.Get("/jd/{jdbh}/yh", s.legacyAgent(http.HandlerFunc(s.handleAgentNodeUsers)).ServeHTTP)
		control.Post("/jd/{jdbh}/yh", s.requireAgentV2(http.HandlerFunc(s.handleAgentV2NodeUsers)).ServeHTTP)
		control.Get("/bg", s.legacyAgent(http.HandlerFunc(s.handleAgentChanges)).ServeHTTP)
		control.Post("/bg", s.requireAgentV2(http.HandlerFunc(s.handleAgentV2Changes)).ServeHTTP)
		control.Post("/yc", s.dualAgentEndpoint(s.handleAgentTelemetry, s.handleAgentV2Telemetry))
		control.Post("/fwq/xt", s.dualAgentEndpoint(s.handleAgentHeartbeat, s.handleAgentV2Heartbeat))
		control.Post("/fwq/jg", s.dualAgentEndpoint(s.handleAgentUpgradeResult, s.handleAgentV2UpgradeResult))
		control.Get("/td", s.handleAgentStreamEntry)
	})

	webHandler := webui.Handler()
	router.With(s.requireAdmin).Handle("/assets/secure/*", webHandler)
	router.With(s.requirePortal).Handle("/assets/portal/*", webHandler)
	router.With(s.requireWebSession).Handle("/assets/shared/*", webHandler)
	router.NotFound(webHandler.ServeHTTP)

	return router
}
