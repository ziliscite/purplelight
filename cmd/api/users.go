package main

import (
	"errors"
	"github.com/ziliscite/purplelight/internal/data"
	"github.com/ziliscite/purplelight/internal/repository"
	"github.com/ziliscite/purplelight/internal/validator"
	"net/http"
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

	err = app.repos.User.Insert(user)
	if err != nil {
		switch {
		// If we get an ErrDuplicateEmail error, use the v.AddError() method to manually
		// add a message to the validator instance, and then call our
		// failedValidationResponse() helper.
		case errors.Is(err, repository.ErrDuplicateEntry):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidation(w, r, v.Errors)
		default:
			app.dbWriteError(w, r, err)
		}
		return
	}

	// Launch a goroutine which runs an anonymous function that sends the welcome email.
	app.background(func() {
		// Call the Send() method on our Mailer, passing in the user's email address,
		// name of the template file, and the User struct containing the new user's data.
		err = app.mailer.Send(user.Email, "user_welcome.tmpl", user)
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
