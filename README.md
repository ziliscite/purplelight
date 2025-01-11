## Chapter 2.3. API Endpoints and RESTful Routing

In this chapter, we are introduced to common http methods, 
like GET, POST, PUT, and DELETE.

This book chooses `httprouter` instead of the standard `net/http`
due to `http.ServeMux` sending plain text responses instead of JSON 
when a matching route cannot be found. Additionally `httprouter` handle OPTIONS requests automatically

Download `httprouter` by
```shell
go get github.com/julienschmidt/httprouter
```

By the end of this chapter, the API endpoints will look like this:  
`GET	/v1/healthcheck	healthcheckHandler` - Show application information  
`POST	/v1/anime	createAnimeHandler` - Create a new anime  
`GET	/v1/anime/:id	showAnimeHandler` - Show the details of a specific anime

Of course, the actual book is doing `movie` instead of `anime`

---
