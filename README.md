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

Then create a [config](#Config) file (by default paste-server will look for it
at `$HOME/.paste.yaml`) containing the MongoDB connection URI for the database
you have set up
```
echo "uri: <your connection URI here>" >> ~/.paste.yaml
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

To set up the server to run as a daemon with systemd see [here](#Daemon)

## Config

The config file is by default `$HOME/.paste.yaml` and is the same file used
both for a paste-server instance and the [paste-cli](https://github.com/h5law/paste-cli)
tool. It can be pointed to any YAML file using the `-c/--config` flag.

The config file MUST contain the `uri` variable - the MongoDB connection string
but can also contain `app_env` a string of either `development` or `test` which
will make the instance use the `LetsEncryptStagingCA` if present otherwise it
will use the `LetsEncryptProductionCA` if `app_env` is not set or set to
anything other than `test` or `development` when using the `-t/--tls` flag.

```
uri: <MongoDB connection uri string>
app_env: <development/test/(production -- optional not needed)>
url: <URL for paste-cli to use if not using the hosted instance at https://pastes.ch>
```

## Daemon

The `paste-server.service` file contains a systemd service script used to
daemonise the paste-server instance. It requires the binary to be built and
moved to `/usr/local/bin/paste-server` and requries the config file containing
the MongoDB connection URI string to be saved at `/etc/paste.yaml` unless
changed.

By default the service file will start on port 80 as root. You will need to
make sure that your firewall has this port open for TCP connections (or for
any port that you use). With firewalld you would run:
```
sudo firewall-cmd --add-port=<PORT>/tcp --permanent
sudo firewall-cmd --reload
sudo firewall-cmd --list-all
```

Make sure that the executable and config files have the right permissions:
```
sudo chown root:root /usr/local/bin/paste-server /etc/paste.yaml
sudo chmod 755 /usr/local/bin/paste-server
sudo restorecon -rv /usr/local/bin/paste-server
sudo chmod 644 /etc/paste.yaml
```

Make sure to run `sudo restorecon -rv /etc/systemd/system/paste-server.service`
after moving the file so that systemd will recognise it - then run
```
sudo systemctl daemon-reload
sudo systemctl start paste-server
sudo systemctl enable paste-server
```

## TLS

When starting the server in TLS mode make sure that you use the `--domain/-d`
and `--email/-e` flags to set the domain of your site and the email to use for
the certificate or the server will not start properly.

Currently HTTP GET requests are forwarded to HTTPS but POST, PUT and DELETE
requests are not.

I used the definition of the [HTTPS function](https://github.com/caddyserver/certmagic/blob/76f61c2947a20d86ca37669dbdc0ed7a96fc6c5f/certmagic.go#L68)
from caddyserver's certmagic github documentation to implement the TLS server
with the ability to gracefully shutdown the server using the same context as
with the http server which was a great help!

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

## URLS + Requests

The paste-server instance will expose the following urls:
 - `/api/new`
 - `/api/{uuid}`
 - `/{uuid}`

The `/api` routes are used by the [paste-cli](https://github.com/h5law/paste-cli)
tool to preform CRUD operations and can be used to send HTTP requests to
interact with the instance without the paste-cli tool.

- `POST /api/new`
  - Requires JSON body containing at least `content` field of an array of
strings, a file split at new-lines
  - Optionally can include `filetype`, and `expiresIn` fields which default to
`plaintext` and `14` respectively
  - Returns a JSON object containing the `accessKey`, `expiresAt`, and `uuid`
fields
- `GET /api/{uuid}`
  - Returns JSON object containing the `content`, `filetype` and `expiresAt`
fields
- `UPDATE /api/{uuid}`
  - Requires JSON body containing any changes to `content`, `filetype`, or a
new `expiresIn` value as well as the `accessKey` field
  - By default the paste will be updated to expire in another 14 days after the
update so use `expriesIn` with any changes made to ensure a longer or shorter
life
- `DELETE /api/{uuid}`
  - Requires the JSON body containing only the `accessKey` field
  - Returns a message confirming the pastes deletion

The `/{uuid}` route is a currently (see [TODO](#TODO)) a simple plaintext site
that displays the result of `GET /api/{uuid}`. However using the query `raw` in
the URL can have the page display only the content of the paste without the
`filetype` and `expiresAt` fields - `/{uuid}?raw=true`

## TODO

- Add subcommand to build preact SPA
- Add front-end site + homepage
- Add ability to create/update/delete pastes from website
- Add flag to disable SPA and use only api+simple plaintext sites
