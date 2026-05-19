package handlers

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

var startTime = time.Now()

type healthResponse struct {
	Status  string `json:"status"`
	Uptime  string `json:"uptime"`
	GoVer   string `json:"go_version"`
	GOOS    string `json:"goos"`
	NumCPU  int    `json:"num_cpu"`
}

type readyResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

// Health is a liveness probe — always 200 as long as the process is running.
func (a *App) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthResponse{
		Status: "ok",
		Uptime: time.Since(startTime).Round(time.Second).String(),
		GoVer:  runtime.Version(),
		GOOS:   runtime.GOOS,
		NumCPU: runtime.NumCPU(),
	})
}

// Ready is a readiness probe — returns 503 if the database is unreachable.
func (a *App) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	dbStatus := "ok"
	if err := a.BusinessStore.Ping(); err != nil {
		dbStatus = "error: " + err.Error()
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(readyResponse{Status: "degraded", DB: dbStatus})
		return
	}
	json.NewEncoder(w).Encode(readyResponse{Status: "ok", DB: dbStatus})
}
