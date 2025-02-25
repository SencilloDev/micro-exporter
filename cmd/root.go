// Copyright 2025 Sencillo
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License

package cmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/SencilloDev/micro-exporter/exporter"
	"github.com/nats-io/jsm.go/natscontext"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "micro-exporter",
	Short: "Prometheus exporter for NATS microservices",
	RunE:  start,
}

var replacer = strings.NewReplacer("-", "_")

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.micro-exporter.yaml)")
	rootCmd.Flags().String("name", "micro-exporter", "connection name")
	viper.BindPFlag("name", rootCmd.Flags().Lookup("name"))
	rootCmd.Flags().Int("port", 10015, "exporter port")
	viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
	rootCmd.Flags().StringP("server", "s", "nats://localhost:4222", "NATS URLs")
	viper.BindPFlag("nats_urls", rootCmd.Flags().Lookup("server"))
	rootCmd.Flags().String("creds", "", "User credentials")
	viper.BindPFlag("credentials_file", rootCmd.Flags().Lookup("creds"))
	rootCmd.Flags().String("jwt", "", "User JWT")
	viper.BindPFlag("nats_jwt", rootCmd.Flags().Lookup("jwt"))
	rootCmd.Flags().String("seed", "", "User seed")
	viper.BindPFlag("nats_seed", rootCmd.Flags().Lookup("seed"))
	rootCmd.Flags().Int("scrape-interval", 15, "Scrape interval to look up new services in seconds")
	viper.BindPFlag("scrape_interval", rootCmd.Flags().Lookup("scrape-interval"))
}

func initConfig() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(replacer)
}

func start(cmd *cobra.Command, args []string) error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	nc, err := newNatsConnection(viper.GetString("name"))
	if err != nil {
		return err
	}

	ex := exporter.New(nc, logger)
	prometheus.MustRegister(ex)
	go ex.WatchForServices(viper.GetInt("scrape_interval"))

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		resp := fmt.Sprintf("<html>" +
			"<head><title>Micro Stats Exporter</title></head>" +
			"<body>\n<h1>Micro Stats Exporter</h1>" +
			"<p><a href='/metrics'>Metrics</a></p>" +
			"</body>\n</html>")
		fmt.Fprint(w, resp)
	})

	port := fmt.Sprintf(":%d", viper.GetInt("port"))
	logger.Info(fmt.Sprintf("starting server on port %s", port))
	return http.ListenAndServe(port, nil)

}

func newNatsConnection(name string) (*nats.Conn, error) {
	opts := []nats.Option{nats.Name(name)}

	_, ok := os.LookupEnv("USER")

	if viper.GetString("credentials_file") == "" && viper.GetString("nats_jwt") == "" && ok {
		slog.Debug("using NATS context")
		return natscontext.Connect("", opts...)
	}

	if viper.GetString("nats_jwt") != "" && viper.GetString("nats_seed") != "" {
		opts = append(opts, nats.UserJWTAndSeed(viper.GetString("nats_jwt"), viper.GetString("nats_seed")))
	}
	if viper.GetString("credentials_file") != "" {
		opts = append(opts, nats.UserCredentials(viper.GetString("credentials_file")))
	}

	return nats.Connect(viper.GetString("nats_urls"), opts...)
}
