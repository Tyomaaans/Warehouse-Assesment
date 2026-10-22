package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"warehouse/internal/auth/session"
	"warehouse/internal/auth/token"
	"warehouse/internal/auth/user"
	"warehouse/internal/config"
	"warehouse/internal/infrastructure/postgres"
	"warehouse/internal/infrastructure/rabbitmq"
	"warehouse/internal/infrastructure/redis"
	"warehouse/internal/inventory"
	"warehouse/internal/middleware"
	"warehouse/internal/notifications"
	"warehouse/internal/products"
	"warehouse/internal/receiving"
	"warehouse/internal/router"
	"warehouse/migrations"
	"warehouse/pkg"
)

func main() {
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("service", "warehouse-api").Logger()
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command == "healthcheck" {
		if err := healthcheck(); err != nil {
			os.Exit(1)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, logger, command); err != nil {
		logger.Error().Err(err).Str("event", "application_stopped").Msg("application failed")
		os.Exit(1)
	}
}

func run(ctx context.Context, logger zerolog.Logger, command string) error {
	if command != "serve" && command != "migrate" && command != "provision-users" && command != "notification-worker" {
		return fmt.Errorf("command must be serve, migrate, provision-users, notification-worker or healthcheck")
	}
	cfg, err := config.NewConfig()
	if err != nil {
		return err
	}
	if command == "serve" {
		if err := cfg.ValidateServer(); err != nil {
			return err
		}
	}
	startup, cancel := context.WithTimeout(ctx, cfg.StartupTimeout)
	defer cancel()
	db, err := postgres.NewPostgresDB(startup, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("PostgreSQL pool unavailable")
	}
	defer sqlDB.Close()
	if command == "migrate" {
		if err := migrations.Run(startup, db); err != nil {
			return fmt.Errorf("migration failed; verify schema and migration checksums")
		}
		logger.Info().Str("event", "migrations_applied").Msg("schema migrations are current")
		return nil
	}
	if err := migrations.Check(startup, db); err != nil {
		return fmt.Errorf("schema is not current; run migrations before serving")
	}
	if command == "provision-users" {
		created, err := user.ProvisionUsers(startup, db, pkg.NewValidator(), []user.ProvisionAccount{
			{Email: os.Getenv("OPERATOR_EMAIL"), Password: os.Getenv("OPERATOR_PASSWORD"), Role: pkg.Operator},
			{Email: os.Getenv("SUPERVISOR_EMAIL"), Password: os.Getenv("SUPERVISOR_PASSWORD"), Role: pkg.Supervisor},
		})
		if err != nil {
			return err
		}
		logger.Info().Str("event", "users_provisioned").Int("created", created).Msg("role accounts provisioned; existing accounts preserved")
		return nil
	}
	if command == "notification-worker" {
		if cfg.RabbitMQURL == "" {
			return fmt.Errorf("RABBITMQ_URL is required for notification-worker")
		}
		notifications.RunWorker(ctx, notifications.NewOutboxRepository(db), notifications.NewNotificationRepository(db), cfg.RabbitMQURL,
			logger.With().Str("component", "notifications").Logger())
		return nil
	}
	redisClient, err := redis.NewRedisClient(startup, cfg.RedisURL, logger)
	if err != nil {
		return err
	}
	defer redisClient.Close()
	rabbitCtx, stopRabbit := context.WithCancel(ctx)
	rabbitDone := make(chan struct{})
	go func() {
		defer close(rabbitDone)
		rabbitmq.MaintainConnection(rabbitCtx, cfg.RabbitMQURL, logger)
	}()
	defer func() {
		stopRabbit()
		<-rabbitDone
	}()
	ready := func(check context.Context) error {
		if err := sqlDB.PingContext(check); err != nil {
			return pkg.ErrUnavailable
		}
		if err := redisClient.Ping(check).Err(); err != nil {
			return pkg.ErrUnavailable
		}
		if err := migrations.Check(check, db); err != nil {
			return pkg.ErrUnavailable
		}
		return nil
	}
	gin.SetMode(gin.ReleaseMode)
	userRepo := user.NewUserRepository(db)
	sessionSvc := session.NewSessionService(redisClient)
	tokenSvc := token.NewJWTService(cfg.JWTSecretKey, cfg.DefaultRefreshExpiry, cfg.ShortRefreshExpiry, redisClient, sessionSvc, userRepo)
	userSvc := user.NewUserService(userRepo, tokenSvc, pkg.NewValidator())
	authHandler := user.NewUserHandler(userSvc, cfg.Environment == "production")
	authMiddleware := middleware.NewAuthMiddleware(tokenSvc)
	productSvc := products.NewProductService(products.NewProductRepository(db), pkg.NewValidator())
	receivingSvc := receiving.NewReceivingService(receiving.NewReceivingRepository(db), pkg.NewValidator())
	inventorySvc := inventory.NewInventoryService(inventory.NewInventoryRepository(db))
	notificationSvc := notifications.NewNotificationService(notifications.NewNotificationRepository(db))
	r, err := router.NewRouter(logger, ready, cfg.TrustedProxies, authHandler, authMiddleware,
		products.NewProductHandler(productSvc), receiving.NewReceivingHandler(receivingSvc), inventory.NewInventoryHandler(inventorySvc), notifications.NewNotificationHandler(notificationSvc))
	if err != nil {
		return fmt.Errorf("invalid trusted proxies")
	}
	server := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	logger.Info().Str("event", "application_started").Str("environment", cfg.Environment).Msg("HTTP server started")
	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP listener failed")
		}
	}
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
		return fmt.Errorf("HTTP shutdown exceeded deadline")
	}
	logger.Info().Str("event", "application_stopped").Msg("HTTP server drained; closing infrastructure")
	return nil
}

func healthcheck() error {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = config.DefaultPort
	}
	client := &http.Client{Timeout: 4 * time.Second}
	response, err := client.Get("http://127.0.0.1:" + port + "/api/v1/ready")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("not ready")
	}
	return nil
}
