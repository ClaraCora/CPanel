package httpapi

import (
	"net/http"

	"cpanel/internal/auth"
	"cpanel/internal/config"
	"cpanel/internal/securebox"
	"cpanel/internal/store"
	webui "cpanel/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const sessionCookieName = "cpanel_admin_session"

type Server struct {
	cfg               config.Config
	store             *store.Store
	dummyPasswordHash string
	secureBox         *securebox.Box
	agentRelease      *agentReleaseResolver
}

func New(cfg config.Config, dataStore *store.Store, box *securebox.Box) *Server {
	dummy, _ := auth.HashPassword("this password is intentionally never valid")
	return &Server{cfg: cfg, store: dataStore, dummyPasswordHash: dummy, secureBox: box, agentRelease: newAgentReleaseResolver(nil, "")}
}

func (s *Server) Handler() http.Handler {
	router := chi.NewRouter()
	router.Use(recoveryMiddleware)
	router.Use(s.commonMiddleware)
	router.Use(middleware.Compress(5))

	router.Get("/health/live", s.handleLive)
	router.Get("/health/ready", s.handleReady)
	router.Get("/corade-downloads/{artifact}", s.handleAgentArtifact)
	router.Post("/api/ops/v1/sessions", s.handleLogin)
	router.Get("/ca/x/{token}", s.handleSubscription)

	router.Route("/api/ops/v1", func(ops chi.Router) {
		ops.Use(s.requireAdmin)
		ops.Get("/session", s.handleCurrentSession)
		ops.Delete("/sessions/current", s.handleLogout)
		ops.Patch("/account/profile", s.handleUpdateAdminProfile)
		ops.Patch("/account/password", s.handleUpdateAdminPassword)
		ops.Post("/tools/reality-keypair", s.handleGenerateRealityCredentials)
		ops.Get("/overview", s.handleOverview)
		ops.Get("/history", s.handleHistoricalData)
		ops.Get("/machines", s.handleListMachines)
		ops.Post("/machines", s.handleCreateMachine)
		ops.Patch("/machines/{id}", s.handleUpdateMachine)
		ops.Delete("/machines/{id}", s.handleDeleteMachine)
		ops.Get("/machines/{id}/installation", s.handleMachineInstallation)
		ops.Post("/machines/{id}/agent-upgrade", s.handleMachineAgentUpgrade)
		ops.Post("/machines/{id}/credentials", s.handleCreateMachineCredential)
		ops.Get("/nodes", s.handleListNodes)
		ops.Post("/nodes", s.handleCreateNode)
		ops.Get("/nodes/{id}", s.handleGetNode)
		ops.Patch("/nodes/{id}", s.handleUpdateNode)
		ops.Delete("/nodes/{id}", s.handleDeleteNode)
		ops.Post("/nodes/{id}/publish", s.handlePublishNode)
		ops.Get("/access-groups", s.handleListAccessGroups)
		ops.Post("/access-groups", s.handleCreateAccessGroup)
		ops.Patch("/access-groups/{id}", s.handleUpdateAccessGroup)
		ops.Delete("/access-groups/{id}", s.handleDeleteAccessGroup)
		ops.Get("/plans", s.handleListPlans)
		ops.Post("/plans", s.handleCreatePlan)
		ops.Patch("/plans/{id}", s.handleUpdatePlan)
		ops.Delete("/plans/{id}", s.handleDeletePlan)
		ops.Get("/users", s.handleListUsers)
		ops.Post("/users", s.handleCreateUser)
		ops.Post("/users/quick/{role}", s.handleQuickCreateUser)
		ops.Get("/users/{id}/subscription", s.handleGetUserSubscription)
		ops.Patch("/users/{id}", s.handleUpdateUser)
		ops.Delete("/users/{id}", s.handleDeleteUser)
		ops.Get("/route-policies", s.handleListRoutePolicies)
		ops.Post("/route-policies", s.handleCreateRoutePolicy)
		ops.Patch("/route-policies/{id}", s.handleUpdateRoutePolicy)
		ops.Delete("/route-policies/{id}", s.handleDeleteRoutePolicy)
		ops.Get("/outbounds", s.handleListOutbounds)
		ops.Post("/outbounds", s.handleCreateOutbound)
		ops.Patch("/outbounds/{id}", s.handleUpdateOutbound)
		ops.Get("/settings/{section}", s.handleListSettings)
		ops.Patch("/settings/{section}", s.handleUpdateSettings)
		ops.Get("/audit-events", s.handleListAuditEvents)
	})

	router.Route("/ca/cc", func(control chi.Router) {
		control.Use(s.requireAgent)
		control.Post("/ws", s.handleAgentHandshake)
		control.Get("/fwq/jd", s.handleAgentNodes)
		control.Get("/jd/{jdbh}/pz", s.handleAgentNodeSpec)
		control.Get("/jd/{jdbh}/yh", s.handleAgentNodeUsers)
		control.Get("/bg", s.handleAgentChanges)
		control.Post("/yc", s.handleAgentTelemetry)
		control.Post("/fwq/xt", s.handleAgentHeartbeat)
		control.Get("/td", s.handleAgentStream)
	})

	router.NotFound(webui.Handler().ServeHTTP)

	return router
}
