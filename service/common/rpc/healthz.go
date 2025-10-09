package rpc

import (
	"net/http"
	"time"

	"github.com/InVisionApp/go-health/v2"
	"github.com/InVisionApp/go-health/v2/checkers"
	"github.com/InVisionApp/go-health/v2/handlers"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"gorm.io/gorm"

	"sentioxyz/sentio-core/common/log"
)

func HealthCheck(db *gorm.DB) runtime.HandlerFunc {
	h := health.New()
	//issuerUrl, _ := url.Parse(config.IssuerURL)
	//issuerUrl.Path = "/.well-known/openid-configuration"

	//auth0Checker, _ := checkers.NewHTTP(&checkers.HTTPConfig{
	//	URL: issuerUrl,
	//})
	conn, _ := db.DB()
	dbChecker, _ := checkers.NewSQL(&checkers.SQLConfig{
		Pinger: conn,
	})

	err := h.AddChecks([]*health.Config{
		//{
		//	Name:     "auth0-check",
		//	Checker:  auth0Checker,
		//	Interval: time.Duration(1) * time.Minute,
		//	Fatal:    false,
		//},
		{
			Name:     "db-check",
			Checker:  dbChecker,
			Interval: time.Duration(5) * time.Second,
			Fatal:    true,
		},
	})
	if err != nil {
		log.Errore(err)
	}
	err = h.Start()
	if err != nil {
		log.Errore(err)
	}
	handler := handlers.NewJSONHandlerFunc(h, nil)
	return func(w http.ResponseWriter, req *http.Request, _ map[string]string) {
		handler.ServeHTTP(w, req)
	}
}

func HealthCheckerWithCheckers(checkers ...*health.Config) (runtime.HandlerFunc, error) {
	h := health.New()
	if err := h.AddChecks(checkers); err != nil {
		return nil, err
	}
	if err := h.Start(); err != nil {
		return nil, err
	}
	handler := handlers.NewJSONHandlerFunc(h, nil)
	return func(w http.ResponseWriter, req *http.Request, _ map[string]string) {
		handler.ServeHTTP(w, req)
	}, nil
}
