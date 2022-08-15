# paste-server

This is a simple Go HTTP server and router that interacts with a MongoDB
database to handle requests that allow for the creation, update, deletion and
reading of "pastes".

Pastes are essentially files stored with some metadata in a MongoDB document
with the following structure:

```
{
    _id:        ObjectId("..."),
    UUID:       String,
    Content:    []String,
    FileType:   String,
    ExpiresAt:  DateTime,
    AccessKey:  String,
}
```

## TODO

- Add verbose support
- Add logging support
    - When no logfile given use `os.Stdout` else redirect all output to logfile
- Automatically convert `content` if given as a string into an array of strings
  seperated on linebreaks

## .env

Your `.env` file should contain a `MONGO_URI` string used to connect to the
databse and an `APP_ENV` string to state what environment the server is being
ran in: `test`, `development` or `production`


## MongoDB

When setting up the database there are a few things you must do, ensure you
are not using the "pastes" database or "files" collection namespaces already as
this is what the server will be using. Secondly create an index as follows:

```
use pastes
db.files.createIndex( { "expiresAt": 1 }, { expireAfterSeconds: 0 } )
```

This will automatically remove pastes when their expiration date is reached.
By default the server will use a period of 14 days but this can be altered.

## Methods

The server supports CRUD operations on the database. Upon creating a paste the
response will contain a field named `accessKey` - this is required for all
`PUT` and `DELETE` operations. Without the access key the paste cannot be
updated or deleted prematurely.

`PUT`, `GET` and `DELETE` http methods for a paste will use the URL:
`/{uuid}` which will find the correct paste based on its unique identifier. But
`POST` uses the URL: `/`. All details of the paste are passed in the request
body. Accepted fields are:

```
{
    "content":      []String,
    "filetype":     String,
    "expiresIn":    Int,
    "accessKey":    String,
}
```
