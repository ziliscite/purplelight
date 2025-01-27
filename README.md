# Chapter 13. Sending Emails

In this section of the book we’re going to inject some interactivity into our API, and adapt our registerUserHandler so that it sends the user a welcome email after they successfully register.

- How to use the Mailtrap SMTP service to send and monitor test emails during development.
- How to use the html/template package and Go’s embedded files functionality to create dynamic and easy-to-manage templates for your email content.
- How to create a reusable internal/mailer package for sending emails from your application.
- How to implement a pattern for sending emails in background goroutines, and how to wait for these to complete during a graceful shutdown.

## SMTP Server Setup
In order to develop our email sending functionality, we’ll need access to a SMTP (Simple Mail Transfer Protocol) server that we can safely use for testing purposes.

There are a huge number of SMTP service providers (such as Postmark, Sendgrid or Amazon SES) that we could use to send our emails — or you can even install and run your own SMTP server. But in this book we’re going to use Mailtrap.

### Setting up Mailtrap
Once you’re registered and logged in, use the menu to navigate to Testing › Inboxes. You should see a page listing your available inboxes.
![img.png](img.png)

Every Mailtrap account comes with one free inbox, which by default is called Demo inbox. You can change the name if you want by clicking the pencil icon under Actions.

If you go ahead and click through to that inbox, you should see that it’s currently empty and contains no emails

Each inbox also has its own set of SMTP credentials.
![img_1.png](img_1.png)

Basically, any emails that you send using these SMTP credentials will end up in this inbox, instead of being sent to the actual recipient.

## Creating Email Templates
To start with, we’ll keep the content of the welcome email really simple, with a short message to let the user know that their registration was successful and confirmation of their ID number.

```markdown
Hi,

Thanks for signing up for a Greenlight account. We're excited to have you on board!

For future reference, your user ID number is 123.

Thanks,

The Greenlight Team
```

Begin by creating a new internal/mailer/templates and then add a user_welcome.tmpl

Inside this file we’re going to define three named templates to use as part of our welcome email:

- A "subject" template containing the subject line for the email.
- A "plainBody" template containing the plain-text variant of the email message body.
- A "htmlBody" template containing the HTML variant of the email message body.

```templ
{{define "subject"}}Welcome to Purplelight!{{end}}

{{define "plainBody"}}
Hi,

Thanks for signing up for a Purplelight account. We're excited to have you on board!

For future reference, your user ID number is {{.ID}}.

Thanks,

The Purplelight Team
{{end}}

{{define "htmlBody"}}
<!doctype html>
<html>

<head>
    <meta name="viewport" content="width=device-width" />
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
</head>

<body>
    <p>Hi,</p>
    <p>Thanks for signing up for a Purplelight account. We're excited to have you on board!</p>
    <p>For future reference, your user ID number is {{.ID}}.</p>
    <p>Thanks,</p>
    <p>The Purplelight Team</p>
</body>

</html>
{{end}} 
```

- We’ve defined the three named templates using the {{define "..."}}...{{end}} tags.
- You can render dynamic data in these templates via the . character (referred to as dot). In the next chapter we’ll pass a User struct to the templates as dynamic data, which means that we can then render the user’s ID using the tag {{.ID}} in the templates.

## Sending a Welcome Email
To send emails we could use Go’s net/smtp package from the standard library. But unfortunately it’s been frozen for a few years, and doesn’t support some of the features that you might need in more advanced use-cases, such as the ability to add attachments.

So instead, we using the third-party go-mail/mail package to help send email.

```shell
go get github.com/go-mail/mail/v2@v2
```

### Creating an email helper
Begin by creating a new internal/mailer/mailer.go
```go
// Below we declare a new variable with the type embed.FS (embedded file system) to hold 
// our email templates. This has a comment directive in the format `//go:embed <path>`
// IMMEDIATELY ABOVE it, which indicates to Go that we want to store the contents of the
// ./templates directory in the templateFS embedded file system variable.
// ↓↓↓

//go:embed "templates"
var templateFS embed.FS

// Define a Mailer struct which contains a mail.Dialer instance (used to connect to a
// SMTP server) and the sender information for your emails (the name and address you
// want the email to be from, such as "Alice Smith <alice@example.com>").
type Mailer struct {
    dialer *mail.Dialer
    sender string
}

func New(host string, port int, username, password, sender string) Mailer {
    // Initialize a new mail.Dialer instance with the given SMTP server settings. We 
    // also configure this to use a 5-second timeout whenever we send an email.
    dialer := mail.NewDialer(host, port, username, password)
    dialer.Timeout = 5 * time.Second

    // Return a Mailer instance containing the dialer and sender information.
    return Mailer{
        dialer: dialer,
        sender: sender,
    }
}

// Define a Send() method on the Mailer type. This takes the recipient email address
// as the first parameter, the name of the file containing the templates, and any
// dynamic data for the templates as an any parameter.
func (m Mailer) Send(recipient, templateFile string, data any) error {
    // Use the ParseFS() method to parse the required template file from the embedded 
    // file system.
    tmpl, err := template.New("email").ParseFS(templateFS, "templates/"+templateFile)
    if err != nil {
        return err
    }

    // Execute the named template "subject", passing in the dynamic data and storing the
    // result in a bytes.Buffer variable.
    subject := new(bytes.Buffer)
    err = tmpl.ExecuteTemplate(subject, "subject", data)
    if err != nil {
        return err
    }

    // Follow the same pattern to execute the "plainBody" template and store the result
    // in the plainBody variable.
    plainBody := new(bytes.Buffer)
    err = tmpl.ExecuteTemplate(plainBody, "plainBody", data)
    if err != nil {
        return err
    }

    // And likewise with the "htmlBody" template.
    htmlBody := new(bytes.Buffer)
    err = tmpl.ExecuteTemplate(htmlBody, "htmlBody", data)
    if err != nil {
        return err
    }

    // Use the mail.NewMessage() function to initialize a new mail.Message instance. 
    // Then we use the SetHeader() method to set the email recipient, sender and subject
    // headers, the SetBody() method to set the plain-text body, and the AddAlternative()
    // method to set the HTML body. It's important to note that AddAlternative() should
    // always be called *after* SetBody().
    msg := mail.NewMessage()
    msg.SetHeader("To", recipient)
    msg.SetHeader("From", m.sender)
    msg.SetHeader("Subject", subject.String())
    msg.SetBody("text/plain", plainBody.String())
    msg.AddAlternative("text/html", htmlBody.String())

    // Call the DialAndSend() method on the dialer, passing in the message to send. This
    // opens a connection to the SMTP server, sends the message, then closes the
    // connection. If there is a timeout, it will return a "dial tcp: i/o timeout"
    // error.
    err = m.dialer.DialAndSend(msg)
    if err != nil {
        return err
    }

    return nil
}
```

### Using embedded file systems
Take a quick moment to discuss embedded file systems in more detail

- You can only use the //go:embed directive on global variables at package level, not within functions or methods. If you try to use it in a function or method, you’ll get the error "go:embed cannot apply to var inside func" at compile time.

- When you use the directive //go:embed "<path>" to create an embedded file system, the path should be relative to the source code file containing the directive. So in our case, //go:embed "templates" embeds the contents of the directory at internal/mailer/templates.

- The embedded file system is rooted in the directory which contains the //go:embed directive. So, in our case, to get the user_welcome.tmpl file we need to retrieve it from templates/user_welcome.tmpl in the embedded file system.

- Paths cannot contain . or .. elements, nor may they begin or end with a /. This essentially restricts you to only embedding files that are contained in the same directory (or a subdirectory) as the source code which has the //go:embed directive.

- If the path is for a directory, then all files in the directory are recursively embedded, except for files with names that begin with . or _. If you want to include these files you should use the * wildcard character in the path, like //go:embed "templates/*"

- You can specify multiple directories and files in one directive. For example: //go:embed "images" "styles/css" "favicon.ico".

- The path separator should always be a forward slash, even on Windows machines.

### Using our mail helper
We need to do two things:

- Adapt our code to accept the configuration settings for the SMTP server as command-line flags.
- Initialize a new Mailer instance and make it available to our handlers via the application struct.

Make sure to use your own Mailtrap SMTP server settings from the previous chapter as the default values for the command line flags here.
```go
// Update the config struct to hold the SMTP server settings.
type config struct {
    port int
    env  string
    db   struct {
        dsn          string
        maxOpenConns int
        maxIdleConns int
        maxIdleTime  time.Duration
    }
    limiter struct {
        enabled bool
        rps     float64
        burst   int
    }
    smtp struct {
        host     string
        port     int
        username string
        password string
        sender   string
    }
}

// Update the application struct to hold a new Mailer instance.
type application struct {
    config config
    logger *slog.Logger
    models data.Models
    mailer mailer.Mailer
}

func main() {
    var cfg config

    flag.IntVar(&cfg.port, "port", 4000, "API server port")
    flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

    flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "PostgreSQL DSN")

    flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
    flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
    flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

    flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")
    flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
    flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")

    // Read the SMTP server configuration settings into the config struct, using the
    // Mailtrap settings as the default values. IMPORTANT: If you're following along,
    // make sure to replace the default values for smtp-username and smtp-password
    // with your own Mailtrap credentials.
    flag.StringVar(&cfg.smtp.host, "smtp-host", "sandbox.smtp.mailtrap.io", "SMTP host")
    flag.IntVar(&cfg.smtp.port, "smtp-port", 25, "SMTP port")
    flag.StringVar(&cfg.smtp.username, "smtp-username", "a7420fc0883489", "SMTP username")
    flag.StringVar(&cfg.smtp.password, "smtp-password", "e75ffd0a3aa5ec", "SMTP password")
    flag.StringVar(&cfg.smtp.sender, "smtp-sender", "Greenlight <no-reply@greenlight.alexedwards.net>", "SMTP sender")

    flag.Parse()

    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

    db, err := openDB(cfg)
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }
    defer db.Close()

    logger.Info("database connection pool established")

    // Initialize a new Mailer instance using the settings from the command line
    // flags, and add it to the application struct.
    app := &application{
        config: cfg,
        logger: logger,
        models: data.NewModels(db),
        mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
    }

    err = app.serve()
    if err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }
}
```

And then the final thing we need to do is update our registerUserHandler to actually send the email, which we can do like so:
```go
func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
    
    ... // Nothing above here needs to change.

    // Call the Send() method on our Mailer, passing in the user's email address,
    // name of the template file, and the User struct containing the new user's data.
    err = app.mailer.Send(user.Email, "user_welcome.tmpl", user)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    err = app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

Try curl
```shell
$ BODY='{"name": "Bob Jones", "email": "bob@example.com", "password": "pa55word"}'
$ curl -w '\nTime: %{time_total}\n' -d "$BODY" localhost:4000/v1/users
{
    "user": {
        "id": 3,
        "created_at": "2021-04-11T20:26:22+02:00",
        "name": "Bob Jones",
        "email": "bob@example.com",
        "activated": false
    }
}

Time: 2.331957
```

> If you receive a "dial tcp: connect: connection refused" error this means that your application could not connect to the Mailtrap SMTP server. Please double check that your Mailtrap credentials are correct, or try using port 2525 instead of port 25.

### Checking the email in Mailtrap
If you’ve been following along and are using the Mailtrap SMTP server credentials, when you go back to your account you should see now the welcome email.
![img_2.png](img_2.png)

### Retrying email send attempts
```go
func (m Mailer) Send(recipient, templateFile string, data any) error {
    ...

    // Try sending the email up to three times before aborting and returning the final 
    // error. We sleep for 500 milliseconds between each attempt.
    for i := 1; i <= 3; i++ {
        err = m.dialer.DialAndSend(msg)
        // If everything worked, return nil.
        if nil == err {
            return nil
        }

        // If it didn't work, sleep for a short time and retry.
        time.Sleep(500 * time.Millisecond)
    }

    return err
}
```

This retry functionality is a relatively simple addition to our code, but it helps to increase the probability that emails are successfully sent in the event of transient network issues.

## Sending Background Emails
Sending the welcome email from the registerUserHandler method adds quite a lot of latency to the total request/response round-trip for the client.

One way we could reduce this latency is by sending the email in a background goroutine.
```go
func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {

    ...

    // Launch a goroutine which runs an anonymous function that sends the welcome email.
    go func() {
        err = app.mailer.Send(user.Email, "user_welcome.tmpl", user)
        if err != nil {
            // Importantly, if there is an error sending the email then we use the 
            // app.logger.Error() helper to manage it, instead of the 
            // app.serverErrorResponse() helper like before.
            app.logger.Error(err.Error())
        }
    }()

    // Note that we also change this to send the client a 202 Accepted status code.
    // This status code indicates that the request has been accepted for processing, but 
    // the processing has not been completed.
    err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

When this code is executed now, a new ‘background’ goroutine will be launched for sending the welcome email. The code in this background goroutine will be executed concurrently with the subsequent code in our registerUserHandler, which means we are no longer waiting for the email to be sent before we return a JSON response to the client.

- We use the app.logger.Error() method to manage any errors in our background goroutine. This is because by the time we encounter the errors, the client will probably have already been sent a 202 Accepted response by our writeJSON() helper.

- Note that we don’t want to use the app.serverErrorResponse() helper to handle any errors in our background goroutine, as that would result in us trying to write a second HTTP response and getting a "http: superfluous response.WriteHeader call" error from our http.Server at runtime.

- The code running in the background goroutine forms a closure over the user and app variables. It’s important to be aware that these ‘closed over’ variables are not scoped to the background goroutine, which means that any changes you make to them will be reflected in the rest of your codebase. For a simple example of this, see the following playground code.

- In our case we aren’t changing the value of these variables in any way, so this behavior won’t cause us any issues. But it is important to keep in mind.

```shell
$ BODY='{"name": "Carol Smith", "email": "carol@example.com", "password": "pa55word"}'
$ curl -w '\nTime: %{time_total}\n' -d "$BODY" localhost:4000/v1/users
{
    "user": {
        "id": 4,
        "created_at": "2021-04-11T21:21:12+02:00",
        "name": "Carol Smith",
        "email": "carol@example.com",
        "activated": false
    }
}

Time: 0.268639
```

This time, you should see that the time taken to return the response is much faster — in his case 0.27 seconds compared to the previous 2.33 seconds.

### Recovering panics
It’s important to bear in mind that any panic which happens in this background goroutine will `not` be automatically recovered by our recoverPanic() middleware or Go’s http.Server, and will cause our whole application to terminate.

The code involved in sending an email is quite complex (including calls to a third-party package) and the risk of a runtime panic is non-negligible.

We need to make sure that any panic in this background goroutine is manually recovered, using a similar pattern to the one in our recoverPanic() middleware.
```go
func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {

    ...

    // Launch a background goroutine to send the welcome email.
    go func() {
        // Run a deferred function which uses recover() to catch any panic, and log an
        // error message instead of terminating the application.
        defer func() {
            if err := recover(); err != nil {
                app.logger.Error(fmt.Sprintf("%v", err))
            }
        }()

        // Send the welcome email.
        err = app.mailer.Send(user.Email, "user_welcome.tmpl", user)
        if err != nil {
            app.logger.Error(err.Error())
        }
    }()

    err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

### Using a helper function
If you need to execute a lot of background tasks in your application, it can get tedious to keep repeating the same panic recovery code — and there’s a risk that you might forget to include it altogether.

To help take care of this, it’s possible to create a simple helper function which wraps the panic recovery logic.

cmd/api/helpers.go
```go
// The background() helper accepts an arbitrary function as a parameter.
func (app *application) background(fn func()) {
    // Launch a background goroutine.
    go func() {
        // Recover any panic.
        defer func() {
            if err := recover(); err != nil {
                app.logger.Error(fmt.Sprintf("%v", err))
            }
        }()

        // Execute the arbitrary function that we passed as the parameter.
        fn()
    }()
}
```

cmd/api/users.go
```go
func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {

    ...
  
    // Use the background helper to execute an anonymous function that sends the welcome
    // email.
    app.background(func() {
        err = app.mailer.Send(user.Email, "user_welcome.tmpl", user)
        if err != nil {
            app.logger.Error(err.Error())
        }
    })

    err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
```

## Graceful Shutdown of Background Tasks
When we initiate a graceful shutdown of our application, it won’t wait for any background goroutines that we’ve launched to complete.

It’s possible that a new client will be created on our system but they will never be sent their welcome email.

### An introduction to sync.WaitGroup
When you want to wait for a collection of goroutines to finish their work, the principal tool to help with this is the sync.WaitGroup type.

The way that it works is conceptually a bit like a ‘counter’. Each time you launch a background goroutine you can increment the counter by 1, and when each goroutine finishes, you then decrement the counter by 1. You can then monitor the counter, and when it equals zero you know that all your background goroutines have finished.
```go
func main() {
    // Declare a new WaitGroup.
    var wg sync.WaitGroup

    // Execute a loop 5 times.
    for i := 1; i <= 5; i++ {
        // Increment the WaitGroup counter by 1, BEFORE we launch the background routine.
        wg.Add(1)

        // Launch the background goroutine.
        go func() {
            // Defer a call to wg.Done() to indicate that the background goroutine has 
            // completed when this function returns. Behind the scenes this decrements 
            // the WaitGroup counter by 1 and is the same as writing wg.Add(-1).
            defer wg.Done()

            fmt.Println("hello from a goroutine")
        }()
    }

    // Wait() blocks until the WaitGroup counter is zero --- essentially blocking until all
    // goroutines have completed.
    wg.Wait()

    fmt.Println("all goroutines finished")
}
```

If you run the above code, you’ll see that the output looks like this:
```shell
hello from a goroutine
hello from a goroutine
hello from a goroutine
hello from a goroutine
hello from a goroutine
all goroutines finished
```

### Fixing application
Update our application to incorporate a sync.WaitGroup that coordinates our graceful shutdown and background goroutines.

```go
// Include a sync.WaitGroup in the application struct. The zero-value for a
// sync.WaitGroup type is a valid, useable, sync.WaitGroup with a 'counter' value of 0,
// so we don't need to do anything else to initialize it before we can use it.
type application struct {
    config config
    logger *slog.Logger
    models data.Models
    mailer mailer.Mailer
    wg     sync.WaitGroup
}
```

File: cmd/api/helpers.go
```go
func (app *application) background(fn func()) {
    // Increment the WaitGroup counter.
    app.wg.Add(1)

    // Launch the background goroutine.
    go func() {
        // Use defer to decrement the WaitGroup counter before the goroutine returns.
        defer app.wg.Done()

        defer func() {
            if err := recover(); err != nil {
                app.logger.Error(fmt.Sprintf("%v", err))
            }
        }()

        fn()
    }()
}
```

File: cmd/api/server.go
```go
func (app *application) serve() error {
    srv := &http.Server{
        Addr:         fmt.Sprintf(":%d", app.config.port),
        Handler:      app.routes(),
        IdleTimeout:  time.Minute,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
    }

    shutdownError := make(chan error)

    go func() {
        quit := make(chan os.Signal, 1)
        signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
        s := <-quit

        app.logger.Info("shutting down server", "signal", s.String())

        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        // Call Shutdown() on the server like before, but now we only send on the
        // shutdownError channel if it returns an error.
        err := srv.Shutdown(ctx)
        if err != nil {
            shutdownError <- err
        }

        // Log a message to say that we're waiting for any background goroutines to
        // complete their tasks.
        app.logger.Info("completing background tasks", "addr", srv.Addr)

        // Call Wait() to block until our WaitGroup counter is zero --- essentially
        // blocking until the background goroutines have finished. Then we return nil on
        // the shutdownError channel, to indicate that the shutdown completed without
        // any issues.
        app.wg.Wait()
        shutdownError <- nil
    }()

    app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

    err := srv.ListenAndServe()
    if !errors.Is(err, http.ErrServerClosed) {
        return err
    }

    err = <-shutdownError
    if err != nil {
        return err
    }

    app.logger.Info("stopped server", "addr", srv.Addr)

    return nil
}
```

Send a request to the POST /v1/users endpoint immediately followed by a SIGTERM signal.
```shell
$ BODY='{"name": "Edith Smith", "email": "edith@example.com", "password": "pa55word"}'
$ curl -d "$BODY" localhost:4000/v1/users & pkill -SIGTERM api &
```

When you do this, your server logs should look similar to the output below:
```shell
$ go run ./cmd/api
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="database connection pool established"
time=2023-09-10T10:59:13.722+02:00 level=INFO msg="starting server" addr=:4000 env=development
time=2023-09-10T10:59:14.722+02:00 level=INFO msg="shutting down server" signal=terminated
time=2023-09-10T10:59:14.722+02:00 level=INFO msg="completing background tasks" addr=:4000
time=2023-09-10T10:59:18.722+02:00 level=INFO msg="stopped server" addr=:4000
```


