package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"cpanel/internal/auth"
	"cpanel/internal/config"
	"cpanel/internal/database"
	"cpanel/internal/httpapi"
	"cpanel/internal/securebox"
	"cpanel/internal/store"
	"cpanel/internal/xboardimport"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(os.Args[1:]); err != nil {
		slog.Error("cpanel stopped", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "serve"
	if len(args) > 0 {
		command = args[0]
		args = args[1:]
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		return err
	}
	dataStore := store.New(pool)
	if err := dataStore.EnsureAdminUsers(ctx); err != nil {
		return fmt.Errorf("ensure administrator subscriptions: %w", err)
	}
	box, err := securebox.New(cfg.EncryptionKey)
	if err != nil {
		return err
	}

	switch command {
	case "serve":
		return serve(cfg, dataStore, box)
	case "migrate":
		slog.Info("database migrations applied")
		return nil
	case "admin":
		return adminCommand(ctx, dataStore, args)
	case "import-xboard":
		return importXboardCommand(ctx, pool, args)
	case "backfill-xboard-subscriptions":
		return backfillXboardSubscriptionsCommand(ctx, pool, args)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func backfillXboardSubscriptionsCommand(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	flags := flag.NewFlagSet("backfill-xboard-subscriptions", flag.ContinueOnError)
	source := flags.String("source", "", "path to the Xboard SQLite database")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*source) == "" {
		return fmt.Errorf("usage: cpanel backfill-xboard-subscriptions --source PATH")
	}
	count, err := xboardimport.New(pool).BackfillSubscriptionTokens(ctx, *source)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]int{"subscription_tokens_backfilled": count})
}

func serve(cfg config.Config, dataStore *store.Store, box *securebox.Box) error {
	maintenanceCtx, stopMaintenance := context.WithCancel(context.Background())
	defer stopMaintenance()
	go maintainHistoricalData(maintenanceCtx, dataStore)
	server := &http.Server{Addr: cfg.Addr, Handler: httpapi.New(cfg, dataStore, box).Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	errCh := make(chan error, 1)
	go func() { slog.Info("cpanel listening", "addr", cfg.Addr); errCh <- server.ListenAndServe() }()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case <-stop:
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func maintainHistoricalData(ctx context.Context, dataStore *store.Store) {
	prune := func() {
		if err := dataStore.PruneHistoricalData(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("historical data retention failed", "error", err)
		}
	}
	prune()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prune()
		}
	}
}

func importXboardCommand(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	flags := flag.NewFlagSet("import-xboard", flag.ContinueOnError)
	source := flags.String("source", "", "path to the Xboard SQLite database")
	friendIDs := flags.String("friend-ids", "5", "comma-separated Xboard user IDs imported as friends")
	skipNodeIDs := flags.String("skip-node-ids", "70", "comma-separated Xboard root node IDs to skip")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*source) == "" {
		return fmt.Errorf("usage: cpanel import-xboard --source PATH [--friend-ids 5] [--skip-node-ids 70]")
	}
	friends, err := parseIDSet(*friendIDs)
	if err != nil {
		return fmt.Errorf("parse --friend-ids: %w", err)
	}
	skipped, err := parseIDSet(*skipNodeIDs)
	if err != nil {
		return fmt.Errorf("parse --skip-node-ids: %w", err)
	}
	result, err := xboardimport.New(pool).Run(ctx, xboardimport.Options{
		SourcePath: *source,
		FriendIDs:  friends,
		SkipNodeID: skipped,
	})
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func parseIDSet(value string) (map[int64]bool, error) {
	result := map[int64]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		id, err := strconv.ParseInt(item, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid ID %q", item)
		}
		result[id] = true
	}
	return result, nil
}

func adminCommand(ctx context.Context, dataStore *store.Store, args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: cpanel admin create --email EMAIL --name NAME --password-env ENV_NAME")
	}
	flags := flag.NewFlagSet("admin create", flag.ContinueOnError)
	email := flags.String("email", "", "administrator email")
	name := flags.String("name", "管理员", "administrator name")
	passwordEnv := flags.String("password-env", "CPANEL_ADMIN_PASSWORD", "environment variable containing the password")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	password := os.Getenv(*passwordEnv)
	if strings.TrimSpace(*email) == "" || password == "" {
		return fmt.Errorf("--email and %s are required", *passwordEnv)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	admin, err := dataStore.CreateAdmin(ctx, *email, *name, hash)
	if err != nil {
		return err
	}
	if err := dataStore.EnsureAdminUsers(ctx); err != nil {
		return fmt.Errorf("create administrator subscription: %w", err)
	}
	fmt.Printf("administrator created: %s (%s)\n", admin.Name, admin.Email)
	return nil
}
