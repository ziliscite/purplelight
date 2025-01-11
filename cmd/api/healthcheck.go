package main

import (
	"encoding/json"
	"net/http"
)

type health struct {
	Status      string `json:"status"`
	Environment string `json:"environment"`
	Version     string `json:"version"`
}

func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	// Create a health struct.
	response := health{
		Status:      "available",
		Environment: app.config.Env(),
		Version:     version,
	}

	// Write a JSON response with a 200 status code.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Encode the health struct as JSON and write it to the response.
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		app.logger.Error("error writing healthcheck response", err)
	}
}
