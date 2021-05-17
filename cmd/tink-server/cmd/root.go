package cmd

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/packethost/pkg/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/tinkerbell/tink/client/listener"
	"github.com/tinkerbell/tink/db"
	rpcServer "github.com/tinkerbell/tink/grpc-server"
	httpServer "github.com/tinkerbell/tink/http-server"
)

// NewRootCommand creates a new Tink Server Cobra root command.
func NewRootCommand(version string, logger log.Logger) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "tink-server",
		Short:   "Tinkerbell provisioning and workflow engine",
		Long:    "Tinkerbell provisioning and workflow engine",
		Version: version,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			viper, err := createViper(logger)
			if err != nil {
				return err
			}

			return applyViper(viper, cmd)
		},
		Run: func(cmd *cobra.Command, args []string) {
			facility, _ := cmd.Flags().GetString("facility")
			caCertFile, _ := cmd.Flags().GetString("ca-cert")
			tlsCertFile, _ := cmd.Flags().GetString("tls-cert")
			tlsKeyFile, _ := cmd.Flags().GetString("tls-key")
			onlyMigration, _ := cmd.Flags().GetBool("only-migration")

			logger = logger.With("facility", facility)
			logger.With("version", version).Info("starting")

			ctx, closer := context.WithCancel(context.Background())
			errCh := make(chan error, 2)

			// TODO(gianarb): I moved this up because we need to be sure that both
			// connection, the one used for the resources and the one used for
			// listening to events and notification are coming in the same way.
			// BUT we should be using the right flags
			connInfo := fmt.Sprintf("dbname=%s user=%s password=%s sslmode=%s",
				os.Getenv("PGDATABASE"),
				os.Getenv("PGUSER"),
				os.Getenv("PGPASSWORD"),
				os.Getenv("PGSSLMODE"),
			)

			dbCon, err := sql.Open("postgres", connInfo)
			if err != nil {
				logger.Fatal(err)
			}

			tinkDB := db.Connect(dbCon, logger)

			if onlyMigration {
				logger.Info("Applying migrations. This process will end when migrations will take place.")
				numAppliedMigrations, err := tinkDB.Migrate()
				if err != nil {
					logger.Fatal(err)
				}
				logger.With("num_applied_migrations", numAppliedMigrations).Info("Migrations applied successfully")
				os.Exit(0)
			}

			if err := listener.Init(connInfo); err != nil {
				logger.Fatal(err)
			}

			go tinkDB.PurgeEvents(errCh)

			numAvailableMigrations, err := tinkDB.CheckRequiredMigrations()
			if err != nil {
				logger.Fatal(err)
			}
			if numAvailableMigrations != 0 {
				logger.Info("Your database schema is not up to date. Please apply migrations running tink-server with env var ONLY_MIGRATION set.")
			}

			tlsCert, certPEM, modT, err := getCerts(caCertFile, tlsCertFile, tlsKeyFile)
			if err != nil {
				logger.Fatal(err)
			}

			rpcServer.SetupGRPC(ctx, logger, facility, tinkDB, certPEM, tlsCert, modT, errCh)
			httpServer.SetupHTTP(ctx, logger, certPEM, modT, errCh)

			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

			select {
			case err = <-errCh:
				logger.Fatal(err)
			case sig := <-sigs:
				logger.With("signal", sig.String()).Info("signal received, stopping servers")
			}
			closer()

			// wait for grpc server to shutdown
			err = <-errCh
			if err != nil {
				logger.Fatal(err)
			}
			err = <-errCh
			if err != nil {
				logger.Fatal(err)
			}
		},
	}

	must := func(err error) {
		if err != nil {
			logger.Fatal(err)
		}
	}

	rootCmd.Flags().String("facility", "", "Facility")

	rootCmd.Flags().String("ca-cert", "", "File containing the ca certificate")

	rootCmd.Flags().String("tls-cert", "bundle.pem", "File containing the tls certificate")
	must(rootCmd.MarkFlagRequired("tls-cert"))

	rootCmd.Flags().String("tls-key", "server-key.pem", "File containing the tls private key")
	must(rootCmd.MarkFlagRequired("tls-cert"))

	rootCmd.Flags().Bool("only-migration", false, "only run database migrations")

	return rootCmd
}

// createViper creates a Viper object configured to read in configuration files
// (from various paths with content type specific filename extensions) and loads
// environment variables.
func createViper(logger log.Logger) (*viper.Viper, error) {
	v := viper.New()
	v.AutomaticEnv()
	v.SetConfigName("tink-server")
	v.AddConfigPath("/etc/tinkerbell")
	v.AddConfigPath(".")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// If a config file is found, read it in.
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			logger.With("configFile", v.ConfigFileUsed()).Error(err, "could not load config file")

			return nil, err
		}

		logger.Info("no config file found")
	} else {
		logger.With("configFile", v.ConfigFileUsed()).Info("loaded config file")
	}

	return v, nil
}

func applyViper(v *viper.Viper, cmd *cobra.Command) error {
	errors := []error{}

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			if err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val)); err != nil {
				errors = append(errors, err)

				return
			}
		}
	})

	if len(errors) > 0 {
		errs := []string{}
		for _, err := range errors {
			errs = append(errs, err.Error())
		}

		return fmt.Errorf(strings.Join(errs, ", "))
	}

	return nil
}

func getCerts(caPath, certPath, keyPath string) (tls.Certificate, []byte, time.Time, error) {
	var (
		modT        time.Time
		caCertBytes []byte
	)

	if caPath != "" {
		ca, modified, err := readFromFile(caPath)
		if err != nil {
			return tls.Certificate{}, nil, modT, fmt.Errorf("failed to read ca cert: %w", err)
		}

		if modified.After(modT) {
			modT = modified
		}

		caCertBytes = ca
	}

	tlsCertBytes, modified, err := readFromFile(certPath)
	if err != nil {
		return tls.Certificate{}, tlsCertBytes, modT, fmt.Errorf("failed to read tls cert: %w", err)
	}

	if modified.After(modT) {
		modT = modified
	}

	tlsKeyBytes, modified, err := readFromFile(keyPath)
	if err != nil {
		return tls.Certificate{}, tlsCertBytes, modT, fmt.Errorf("failed to read tls key: %w", err)
	}

	if modified.After(modT) {
		modT = modified
	}

	// If we read in a separate ca certificate, concatenate it with the tls cert
	if len(caCertBytes) > 0 {
		tlsCertBytes = append(tlsCertBytes, caCertBytes...)
	}

	cert, err := tls.X509KeyPair(tlsCertBytes, tlsKeyBytes)
	if err != nil {
		return cert, tlsCertBytes, modT, fmt.Errorf("failed to ingest TLS files: %w", err)
	}

	return cert, tlsCertBytes, modT, nil
}

func readFromFile(filePath string) ([]byte, time.Time, error) {
	var modified time.Time

	f, err := os.Open(filePath)
	if err != nil {
		return nil, modified, err
	}

	stat, err := f.Stat()
	if err != nil {
		return nil, modified, err
	}

	modified = stat.ModTime()

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, modified, err
	}

	return contents, modified, nil
}
