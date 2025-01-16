package main

import (
	"net/http"
)

func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	response := struct {
		Environment string `json:"environment"`
		Version     string `json:"version"`
	}{
		Environment: app.config.Env(),
		Version:     version,
	}

	env := envelope{
		"status":      "available",
		"system_info": response,
	}

	err := app.write(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverError(w, r, err)
	}
}
