package main

import (
	"errors"
	"github.com/ziliscite/purplelight/internal/data"
	"github.com/ziliscite/purplelight/internal/repository"
	"github.com/ziliscite/purplelight/internal/validator"
	"net/http"
	"time"
)

func (app *application) createActivationToken(w http.ResponseWriter, r *http.Request) {
	// Parse and validate the user's email address.
	var input struct {
		Email string `json:"email"`
	}

	err := app.readBody(w, r, &input)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateEmail(v, input.Email); !v.Valid() {
		app.failedValidation(w, r, v.Errors)
		return
	}

	// Try to retrieve the corresponding user record for the email address. If it can't
	// be found, return an error message to the client.
	user, err := app.repos.User.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrRecordNotFound):
			v.AddError("email", "no matching email address found")
			app.failedValidation(w, r, v.Errors)
		default:
			app.dbReadError(w, r, err)
		}
		return
	}

	// Return an error if the user has already been activated.
	if user.Activated {
		v.AddError("email", "user has already been activated")
		app.failedValidation(w, r, v.Errors)
		return
	}

	// Otherwise, create a new activation token.
	token, err := app.repos.Token.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.dbWriteError(w, r, err)
		return
	}

	// Email the user with their additional activation token.
	app.background(func() {
		tokenData := map[string]any{
			"activationToken": token.Plaintext,
		}

		// Since email addresses MAY be case sensitive, notice that we are sending this
		// email using the address stored in our database for the user --- not to the
		// input.Email address provided by the client in this request.
		err = app.mailer.Send(user.Email, "token_activation.tmpl", tokenData)
		if err != nil {
			app.logger.Error(err.Error())
		}
	})

	// Send a 202 Accepted response and confirmation message to the client.
	err = app.write(w, http.StatusAccepted, envelope{"message": "an email will be sent to you containing activation instructions"}, nil)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) activateUser(w http.ResponseWriter, r *http.Request) {
	// Parse the plaintext activation token from the request body.
	var input struct {
		TokenPlaintext string `json:"token"`
	}

	err := app.readBody(w, r, &input)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	// Validate the plaintext token provided by the client.
	v := validator.New()

	if data.ValidateTokenPlaintext(v, input.TokenPlaintext); !v.Valid() {
		app.failedValidation(w, r, v.Errors)
		return
	}

	// Retrieve the details of the user associated with the token using the
	// GetForToken() method (which we will create in a minute). If no matching record
	// is found, then we let the client know that the token they provided is not valid.
	user, err := app.repos.User.GetForToken(data.ScopeActivation, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidation(w, r, v.Errors)
		default:
			app.dbReadError(w, r, err)
		}
		return
	}

	// Update the user's activation status.
	user.Activated = true

	// Save the updated user record in our database, checking for any edit conflicts in
	// the same way that we did for our movie records.
	err = app.repos.User.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrEditConflict):
			app.editConflict(w, r)
		default:
			app.dbWriteError(w, r, err)
		}
		return
	}

	// don't we usually want to use a transaction for this?

	// If everything went successfully, then we delete all activation tokens for the
	// user.
	err = app.repos.Token.DeleteAllForUser(data.ScopeActivation, user.ID) // what if this fails?
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// Send the updated user details to the client in a JSON response.
	err = app.write(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		app.serverError(w, r, err)
	}
}
