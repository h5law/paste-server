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

## Install

Before you can run an instance of the paste-server locally you must first
set up and configure (only one command) a MongoDB instance see [here](#MongoDB).


First clone this repo and enter into the directory

```
git clone https://github.com/h5law/paste-server
cd paste-server
```

Then create a `.env` file in the root directory containing the MongoDB
connection URI for the database you have set up
```
echo "MONGO_URI=<your connection URI here>" > .env
```

Then install all dependencies
```
go mod tidy
```

Finally build the binary
```
go build -o paste-server
```

Now you can run `./paste-server` to see detailed usage info or to just simply
run the server
```
./paste-server start
```

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

## TODO

- Seperate route handler into 2 sections `/api` and `/{uuid}`
- `/api` will act as MongoDB interaction routes
- `/{uuid}` will be a static site generator to view and interact with
pastes
