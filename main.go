package main

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	port := getEnvDefault("PORT", "8080")
	healthcheckPath := getEnvDefault("HEALTHCHECK_PATH", "/healthz")
	terminationDelay := getEnvDurationDefault("TERMINATION_DELAY", time.Second*120)

	upstreamPort := requireEnv("UPSTREAM_PORT")
	upstreamHealthcheckPath := requireEnv("UPSTREAM_HEALTHCHECK_PATH")
	upstreamTimeout := requireEnvDuration("UPSTREAM_TIMEOUT")
	upstreamHealth := fmt.Sprintf("http://localhost:%s%s", upstreamPort, upstreamHealthcheckPath)

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	signals := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	terminating := false

	go func() {
		sig := <-signals
		log.Printf("caught signal %s", sig)
		log.Print("will return unhealthy response")
		terminating = true

		log.Printf("terminating in %v", terminationDelay)
		t := time.NewTimer(terminationDelay)
		select {
		case <-t.C:
			log.Printf("timeout expired, exiting")
			done <- true
		}
	}()

	e.GET(healthcheckPath, func(c echo.Context) error {
		if terminating {
			return c.String(http.StatusServiceUnavailable, "terminating")
		}

		client := http.Client{
			Timeout: upstreamTimeout,
		}

		log.Printf("checking upstream health at %s", upstreamHealth)
		res, err := client.Get(upstreamHealth)
		if err != nil {
			log.Print("failed to call upstream")
			return c.String(http.StatusServiceUnavailable, "upstream service is not available")
		}
		if res.StatusCode >= http.StatusInternalServerError {
			log.Printf("upstream returned error: %d", res.StatusCode)
			return c.String(http.StatusServiceUnavailable, "upstream service is returning error")
		}

		log.Print("upstream is healthy")
		return c.String(http.StatusOK, "OK")
	})

	go func() {
		log.Fatal(e.Start(":" + port))
	}()

	<-done
}

func getEnvDurationDefault(name string, defaultValue time.Duration) time.Duration {
	value, ok := os.LookupEnv(name)
	if !ok {
		return defaultValue
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("error parsing %s: %s", name, err)
		return defaultValue
	}
	return duration
}

func getEnvDefault(name string, defaultValue string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return defaultValue
	}
	return value
}

func requireEnv(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		log.Fatalf("%s is not set", name)
	}
	return value
}

func requireEnvDuration(name string) time.Duration {
	value, ok := os.LookupEnv(name)
	if !ok {
		log.Fatalf("%s is not set", name)
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Fatalf("error parsing %s: %s", name, err)
	}
	return duration
}
