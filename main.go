package main

import (
	"fmt"
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

	http.HandleFunc(healthcheckPath, func(w http.ResponseWriter, r *http.Request) {
		if terminating {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("terminating"))
			return
		}

		client := http.Client{
			Timeout: upstreamTimeout,
		}
		res, err := client.Get(upstreamHealth)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("upstream service is not available"))
			return
		}
		if res.StatusCode >= http.StatusInternalServerError {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("upstream service is returning error"))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	go func() {
		log.Printf("listening healthcheck requests at :%s", port)
		log.Fatal(http.ListenAndServe(":"+port, nil))
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
