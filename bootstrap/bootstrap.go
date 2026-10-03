package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	sharedauth "github.com/gotcli/libs/auth"
	"github.com/gotcli/libs/config"
	"github.com/gotcli/libs/database"
	"github.com/gotcli/libs/logger"
	"github.com/gotcli/libs/middlewares"
	"github.com/gotcli/libs/response"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type Options struct {
	// Module is the service's Go module path; it names the service and sets
	// the environment variable prefix.
	Module string
	// Defaults adds or overrides configuration defaults for this service.
	Defaults map[string]any
	// RequireJWT builds the access-token verifier before connecting to the
	// database, so a missing or weak JWT_ACCESS_SECRET stops startup.
	RequireJWT bool
	// Validate runs after configuration loads for service-specific checks.
	Validate func() error
	// Database returns the service's connection settings. It is called after
	// configuration loads, so it may read viper. Required.
	Database func() database.Config
	// AllowedOrigins returns the browser origins allowed to call the service
	// directly (CORS). It is called after configuration loads; nil or an empty
	// list leaves CORS off.
	AllowedOrigins func() []string
	// Register mounts the service routes. Health routes are already mounted.
	Register func(app *fiber.App, deps Deps) error
}

// Deps are the shared resources handed to Register.
type Deps struct {
	DB       *gorm.DB
	Verifier *sharedauth.Verifier // nil unless Options.RequireJWT
	Loggers  *logger.Loggers
}

// Run starts the service and blocks until SIGINT/SIGTERM or a server error.
// It returns the process exit code.
func Run(options Options) int {
	if err := run(options); err != nil {
		log.Printf("%s: %v", config.EnvPrefix(options.Module), err)
		return 1
	}
	return 0
}

func run(options Options) error {
	if options.Module == "" || options.Register == nil || options.Database == nil {
		return errors.New("bootstrap: Module, Database and Register are required")
	}
	if err := config.Load(options.Module, options.Defaults); err != nil {
		return fmt.Errorf("initialize config: %w", err)
	}
	if options.Validate != nil {
		if err := options.Validate(); err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}
	}
	databaseConfig := options.Database()
	if err := databaseConfig.Validate(); err != nil {
		return err
	}
	var allowCORS fiber.Handler
	if options.AllowedOrigins != nil {
		if origins := options.AllowedOrigins(); len(origins) > 0 {
			handler, err := corsMiddleware(origins)
			if err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}
			allowCORS = handler
		}
	}
	deps := Deps{}
	if options.RequireJWT {
		verifier, err := sharedauth.NewVerifier(viper.GetString("JWT_ISSUER"), viper.GetString("JWT_AUDIENCE"), viper.GetString("JWT_ACCESS_SECRET"))
		if err != nil {
			return fmt.Errorf("initialize JWT verifier: %w", err)
		}
		deps.Verifier = verifier
	}
	loggers, err := logger.Setup(viper.GetString("LOG_DIR"))
	if err != nil {
		return fmt.Errorf("initialize logging: %w", err)
	}
	defer loggers.Close()
	deps.Loggers = loggers

	shutdown, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	deps.DB, err = database.Connect(shutdown, databaseConfig)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer database.Close(deps.DB)

	app := NewApp(loggers)
	if allowCORS != nil {
		// Before every route, so preflight requests never reach JWT checks.
		app.Use(allowCORS)
	}
	RegisterHealthRoutes(app, deps.DB)
	if err := options.Register(app, deps); err != nil {
		return fmt.Errorf("register routes: %w", err)
	}
	return serve(shutdown, app)
}

// NewApp creates a fiber app with the shared limits, error envelope, request
// logging and panic recovery.
func NewApp(loggers *logger.Loggers) *fiber.App {
	app := fiber.New(fiber.Config{
		ReadBufferSize: 8192, BodyLimit: viper.GetInt("HTTP_BODY_LIMIT"), ErrorHandler: response.ErrorHandler,
		ReadTimeout: viper.GetDuration("HTTP_READ_TIMEOUT"), WriteTimeout: viper.GetDuration("HTTP_WRITE_TIMEOUT"),
		IdleTimeout: viper.GetDuration("HTTP_IDLE_TIMEOUT"),
	})
	if loggers != nil {
		app.Use(middlewares.RequestLogger(loggers))
	}
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))
	return app
}

func serve(shutdown context.Context, app *fiber.App) error {
	serverError := make(chan error, 1)
	go func() { serverError <- app.Listen(fmt.Sprintf(":%d", viper.GetInt("APP_PORT"))) }()
	var result error
	select {
	case <-shutdown.Done():
	case err := <-serverError:
		if err != nil {
			result = fmt.Errorf("server stopped: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration("SHUTDOWN_TIMEOUT"))
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil && result == nil {
		result = fmt.Errorf("graceful shutdown failed: %w", err)
	}
	return result
}
