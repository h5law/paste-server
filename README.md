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

Then create a config file (by default paste-server will look for it at
`$HOME/.paste.yaml`) containing the MongoDB connection URI for the database you
have set up
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
    - ~~`/api` will act as MongoDB interaction routes~~
    - `/{uuid}` will be a static site generator to view and interact with
pastes
- Add `-s` and `--secure` flags to `start` subcommand to use TLS
