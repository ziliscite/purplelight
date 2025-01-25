# Chapter 10. Rate Limiting
If you’re building an API for public use, then it’s quite likely that you’ll want to implement some form of rate limiting to prevent clients from making too many requests too quickly, and putting excessive strain on your server.

In this section of the book we’re going to create some middleware to help with that.

Essentially, we want this middleware to check how many requests have been received in the last ‘N’ seconds and — if there have been too many — then it should send the client a 429 Too Many Requests response. We’ll position this middleware before our main application handlers, so that it carries out this check before we do any expensive processing like decoding a JSON request body or querying our database.

- About the principles behind token-bucket rate-limiter algorithms and how we can apply them in the context of an API or web application.
- How to create middleware to rate-limit requests to your API endpoints, first by making a single rate global limiter, then extending it to support per-client limiting based on IP address.
- How to make rate limiter behavior configurable at runtime, including disabling the rate limiter altogether for testing purposes.

## Global Rate Limiting

