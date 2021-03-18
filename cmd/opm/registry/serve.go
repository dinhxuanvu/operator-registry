package registry

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	mode "github.com/operator-framework/operator-registry/cmd/opm/registry/internal/mode"
	"github.com/operator-framework/operator-registry/pkg/api"
	health "github.com/operator-framework/operator-registry/pkg/api/grpc_health_v1"
	"github.com/operator-framework/operator-registry/pkg/declcfg"
	"github.com/operator-framework/operator-registry/pkg/lib/dns"
	"github.com/operator-framework/operator-registry/pkg/lib/graceful"
	"github.com/operator-framework/operator-registry/pkg/lib/log"
	"github.com/operator-framework/operator-registry/pkg/lib/tmp"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/server"
	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

type serve struct {
	debug          bool
	database       string
	port           string
	terminationLog string
	skipMigrate    bool
	timeout        string

	logger *logrus.Entry

	objectsDirectory string
}

func newRegistryServeCmd() *cobra.Command {
	s := serve{
		logger: logrus.NewEntry(logrus.StandardLogger()),
	}
	rootCmd := &cobra.Command{
		Use:   "serve <source_path>",
		Short: "serve an operator-registry source",
		Long:  `serve an operator-registry source that is queryable using grpc`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if s.debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return s.run(cmd.Context())
		},
	}

	rootCmd.Flags().BoolVar(&s.debug, "debug", false, "enable debug logging")
	rootCmd.Flags().StringVarP(&s.database, "database", "d", "bundles.db", "relative path to sqlite db")
	rootCmd.Flags().StringVarP(&s.port, "port", "p", "50051", "port number to serve on")
	rootCmd.Flags().StringVarP(&s.terminationLog, "termination-log", "t", "/dev/termination-log", "path to a container termination log file")
	rootCmd.Flags().BoolVar(&s.skipMigrate, "skip-migrate", false, "do  not attempt to migrate to the latest db revision when starting")
	rootCmd.Flags().StringVar(&s.timeout, "timeout-seconds", "infinite", "Timeout in seconds. This flag will be removed later.")
	rootCmd.Flags().StringVarP(&s.objectsDirectory, "objects", "o", "objects", "relative path to directory containing bundle objects")
	return rootCmd
}

func (s *serve) run(ctx context.Context) error {
	// Immediately set up termination log
	err := log.AddDefaultWriterHooks(s.terminationLog)
	if err != nil {
		logrus.WithError(err).Warn("unable to set termination log path")
	}

	// Ensure there is a default nsswitch config
	if err := dns.EnsureNsswitch(); err != nil {
		logrus.WithError(err).Warn("unable to write default nsswitch config")
	}

	s.logger = s.logger.WithFields(logrus.Fields{"database": s.database, "port": s.port})

	dbMode, err := mode.DetectSourceMode(s.database)
	if err != nil {
		return fmt.Errorf("could not detect source mode for file %q: %v", s.database, err)
	}

	var (
		store    registry.GRPCQuery
		storeErr error
	)
	switch dbMode {
	case mode.ModeDeclCfgDir:
		cfg, err := declcfg.LoadDir(s.database)
		if err != nil {
			return fmt.Errorf("load declarative config directory: %v", err)
		}
		store, storeErr = declcfgQuerier(*cfg, s.objectsDirectory)
	case mode.ModeDeclCfgFile:
		cfg, err := declcfg.LoadFile(s.database)
		if err != nil {
			return fmt.Errorf("load declarative config file: %v", err)
		}
		store, storeErr = declcfgQuerier(*cfg, s.objectsDirectory)
	case mode.ModeDeclCfgTar:
		cfg, err := declcfg.LoadTar(s.database)
		if err != nil {
			return fmt.Errorf("load declarative config tar: %v", err)
		}
		store, storeErr = declcfgQuerier(*cfg, s.objectsDirectory)
	case mode.ModeSqlite:
		// make a writable copy of the db for migrations
		tmpdb, err := tmp.CopyTmpDB(s.database)
		if err != nil {
			return err
		}
		defer os.Remove(tmpdb)
		store, storeErr = s.loadDBStore(ctx, tmpdb)
	default:
		return fmt.Errorf("unexpected registry source mode %q", dbMode)
	}
	if storeErr != nil {
		return fmt.Errorf("failed to load grpc store: %v", storeErr)
	}

	lis, err := net.Listen("tcp", ":"+s.port)
	if err != nil {
		s.logger.Fatalf("failed to listen: %s", err)
	}

	grpcServer := grpc.NewServer()
	s.logger.Printf("Keeping server open for %s seconds", s.timeout)
	if s.timeout != "infinite" {
		timeoutSeconds, err := strconv.ParseUint(s.timeout, 10, 16)
		if err != nil {
			return err
		}

		timeoutDuration := time.Duration(timeoutSeconds) * time.Second
		timer := time.AfterFunc(timeoutDuration, func() {
			s.logger.Info("Timeout expired. Gracefully stopping.")
			grpcServer.GracefulStop()
		})
		defer timer.Stop()
	}

	api.RegisterRegistryServer(grpcServer, server.NewRegistryServer(store))
	health.RegisterHealthServer(grpcServer, server.NewHealthServer())
	reflection.Register(grpcServer)
	s.logger.Info("serving registry")
	return graceful.Shutdown(s.logger, func() error {
		return grpcServer.Serve(lis)
	}, func() {
		grpcServer.GracefulStop()
	})
}

func declcfgQuerier(cfg declcfg.DeclarativeConfig, objectsDir string) (registry.GRPCQuery, error) {
	m, err := declcfg.ConvertToModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not build index model from declarative config: %v", err)
	}
	q := registry.NewQuerier(m)
	if err := q.LoadBundleObjects(objectsDir); err != nil {
		return nil, fmt.Errorf("could not load bundle objects from dir %q: %v", objectsDir, err)
	}
	return q, nil
}

func (s *serve) loadDBStore(ctx context.Context, source string) (registry.GRPCQuery, error) {
	db, err := sqlite.Open(source)
	if err != nil {
		return nil, err
	}

	if !s.skipMigrate {
		// migrate to the latest version
		if err := s.migrate(ctx, db); err != nil {
			s.logger.WithError(err).Warnf("couldn't migrate db")
		}
	}

	dbStore := sqlite.NewSQLLiteQuerierFromDb(db)

	// sanity check that the db is available
	tables, err := dbStore.ListTables(ctx)
	if err != nil {
		s.logger.WithError(err).Warnf("couldn't list tables in db")
	}
	if len(tables) == 0 {
		s.logger.Warn("no tables found in db")
	}

	if s.skipMigrate {
		return dbStore, nil
	}

	objs, err := sqlite.GetBundleObjects(ctx, dbStore)
	if err != nil {
		return nil, err
	}

	m, err := sqlite.ToModel(ctx, dbStore)
	if err != nil {
		return nil, err
	}
	q := registry.NewQuerier(m)
	q.SetBundleObjects(objs)

	return q, nil
}

func (s serve) migrate(ctx context.Context, db *sql.DB) error {
	migrator, err := sqlite.NewSQLLiteMigrator(db)
	if err != nil {
		return err
	}
	if migrator == nil {
		return fmt.Errorf("failed to load migrator")
	}

	return migrator.Migrate(ctx)
}
