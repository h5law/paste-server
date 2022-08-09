package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Paste struct {
	ID        *primitive.ObjectID `josn:"ID" bson:"_id,omitempty"`
	UUID      uuid.UUID           `json:"uuid" bson:"uuid,omitempty"`
	Name      string              `json:"name" bson:"name,omitempty"`
	Content   string              `json:"content" bson:"content,omitempty"`
	FileType  string              `json:"filetype" bson:"filetype,omitempty"`
	AccessKey string              `json:"access_key" bson:"acces_key,omitempty"`
	Expires   string              `json:"expires" bson:"expires,omitempty"`
}

type Server struct {
	*mux.Router
	*mongo.Client
}

func (s *Server) ConnectDB(uri string) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("connected to database")
	s.Client = client
}

func (s *Server) DisconnectDB() {
	if err := s.Client.Disconnect(context.TODO()); err != nil {
		log.Fatal("error disconnecting from database")
	}
	log.Println("disconnected from database")
	s.Client = nil
}

func (s *Server) routes() {
	s.HandleFunc("/", s.createPaste()).Methods("POST")
	s.HandleFunc("/{uuid}", s.getPaste()).Methods("GET")
	s.HandleFunc("/{uuid}", s.updatePaste()).Methods("PUT")
	s.HandleFunc("/{uuid}", s.deletePaste()).Methods("DELETE")
}

func NewServer(uri string) *Server {
	s := &Server{
		Router: mux.NewRouter(),
	}
	s.ConnectDB(uri)
	if err := s.Client.Ping(nil, nil); err != nil {
		log.Fatal("error connecting to database")
	}
	s.routes()
	return s
}

/* POST /
r.Body:
	"content"  -> required
	"name"     -> optional
	"filetype" -> optional

Creates a new Paste in the MongoDB database and returns a JSON document
{
	uuid:		UUID,
	name:		String,
	content:	String,
	filetype:	String,
	access_key: String,
	expires:	Date
}
*/
func (s *Server) createPaste() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			log.Printf("%s %s [%v]\n",
				r.Method,
				r.URL.Path,
				time.Since(start),
			)
		}()
		fmt.Fprintf(w, "POST /")
	}
}

/* GET /{uuid}

Returns the Paste from the MongoDB database with the matching UUID in JSON
{
	uuid:		UUID,
	name:		String,
	content:	String,
	filetype:	String,
	access_key: String,
	expires:	Date
}
*/
func (s *Server) getPaste() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		uuidStr, _ := mux.Vars(r)["uuid"]
		defer func() {
			log.Printf("%s %s [%v]\n",
				r.Method,
				r.URL.Path,
				time.Since(start),
			)
		}()

		coll := s.Client.Database("pastes").Collection("files")
		var result bson.M
		filter := bson.D{{Key: "uuid", Value: uuidStr}}
		project := bson.D{{Key: "_id", Value: 0}, {Key: "access_key", Value: 0}}

		err := coll.FindOne(
			context.TODO(),
			filter,
			options.FindOne().SetProjection(project),
		).Decode(&result)

		if err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			panic(err)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

/* PUT /{uuid}
r.Body:
	"access_key"  -> required
	"content"	  -> optional
	"name"        -> optional
	"filetype"    -> optional
	^ At least one of the 3 optional fields must be updated

Updates an existing Paste in the MongoDB database and returns a JSON document
{
	uuid:		UUID,
	name:		String,
	content:	String,
	filetype:	String,
	access_key: String,
	expires:	Date
}
*/
func (s *Server) updatePaste() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			log.Printf("%s %s [%v]\n",
				r.Method,
				r.URL.Path,
				time.Since(start),
			)
		}()
		fmt.Fprintf(w, "PUT /{uuid}")
	}
}

/* DELETE /{uuid}
r.Body:
	"access_key"  -> required

Deletes an existing Paste in the MongoDB database
*/
func (s *Server) deletePaste() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			log.Printf("%s %s [%v]\n",
				r.Method,
				r.URL.Path,
				time.Since(start),
			)
		}()
		fmt.Fprintf(w, "DELETE /{uuid}")
	}
}
