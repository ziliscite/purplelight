package main

import (
	"errors"
	"github.com/ziliscite/purplelight/internal/data"
	"github.com/ziliscite/purplelight/internal/repository"
	"github.com/ziliscite/purplelight/internal/validator"
	"net/http"
	"time"
)

func (app *application) registerUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readBody(w, r, &input)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	// Copy the data from the request body into a new User struct. Notice also that we
	// set the Activated field to false, which isn't strictly necessary because the
	// Activated field will have the zero-value of false by default. But setting this
	// explicitly helps to make our intentions clear to anyone reading the code.
	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}

	// Use the Password.Set() method to generate and store the hashed and plaintext
	// passwords.
	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidation(w, r, v.Errors)
		return
	}

	// Yo chat, I might need to use transactions among these 3 operations
	// It'll be real disaster if user is inserted, but then it fails midway
	// token will not be sent, and the necessary permissions will not be granted...

	// TODO: Refactor the codebase to use a service layer so we can manage transactions between these 3 repositories
	// For other handlers as well

	err = app.repos.User.Insert(user)
	if err != nil {
		switch {
		// If we get an ErrDuplicateEmail error, use the v.AddError() method to manually
		// add a message to the validator instance
		case errors.Is(err, repository.ErrDuplicateEntry):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidation(w, r, v.Errors)
		default:
			app.dbWriteError(w, r, err)
		}
		return
	}

	// Add the "movies:read" permission for the new user.
	err = app.repos.Permission.AddForUser(user.ID, "anime:read")
	if err != nil {
		app.dbWriteError(w, r, err)
		return
	}

	// After the user record has been created in the database, generate a new activation
	// token for the user.
	token, err := app.repos.Token.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.dbWriteError(w, r, err)
		return
	}

	// Launch a goroutine which runs an anonymous function that sends the welcome email.
	app.background(func() {
		// As there are now multiple pieces of data that we want to pass to our email
		// templates, we create a map to act as a 'holding structure' for the data. This
		// contains the plaintext version of the activation token for the user, along
		// with their ID.
		userData := map[string]any{
			"activationToken": token.Plaintext,
			"userID":          user.ID,
		}

		// Call the Send() method on our Mailer, passing in the user's email address,
		// name of the template file, and the User struct containing the new user's data.
		err = app.mailer.Send(user.Email, "user_welcome.tmpl", userData)
		if err != nil {
			// Importantly, if there is an error sending the email then we use the
			// app.logger.Error() helper to manage it, instead of the
			// app.serverErrorResponse() helper like before.
			app.logger.Error(err.Error())
		}
	})

	err = app.write(w, http.StatusCreated, envelope{"user": user}, nil)
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
	// GetForToken() method. If no matching record
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
