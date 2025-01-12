package main

import (
	"net/http"
)

func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	// Create a struct to encode json.
	response := struct {
		Environment string `json:"environment"`
		Version     string `json:"version"`
	}{
		Environment: app.config.Env(),
		Version:     version,
	}

	// Declare an envelope map containing the data for the response. Notice that the way
	// we've constructed this means the environment and version data will now be nested
	// under a system_info key in the JSON response.
	env := envelope{
		"status":      "available",
		"system_info": response,
	}

	// Write a JSON response with a 200 status code.
	// Encode the health struct as JSON and write it to the response.
	err := app.write(w, http.StatusOK, env, nil)
	if err != nil {
		// Use the new serverErrorResponse() helper.
		app.serverError(w, r, err)
	}

	// I usually used this sequence to write the response
	//err := json.NewEncoder(w).Encode(response)
	//if err != nil {
	//	app.logger.Error("error writing healthcheck response", err)
	//  http.Error(w, "The server encountered a problem and could not process your request", http.StatusInternalServerError)
	//}

	// this is an option, by the way
	//js := `{"status": "available", "environment": %q, "version": %q}`
	//js = fmt.Sprintf(js, app.config.env, version)
}
