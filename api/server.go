package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/golang/gddo/httputil/header"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Server struct {
	*mux.Router
	*mongo.Client
}

func (s *Server) ConnectDB(uri string) {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	s.Client = client
	log.Println("connected to database")
}

func (s *Server) DisconnectDB() {
	if err := s.Client.Disconnect(context.TODO()); err != nil {
		log.Fatal("error disconnecting from database")
	}
	s.Client = nil
	log.Println("disconnected from database")
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

type PasteBody struct {
	Name      string `json:"name,omitempty"`
	Content   string `json:"content"`
	FileType  string `json:"filetype,omitempty"`
	ExpiresIn int    `json:"expiresIn,omitempty"`
}

type Paste struct {
	UUID      string             `json:"uuid,omitempty" bson:"uuid,omitempty"`
	Name      string             `json:"name,omitempty" bson:"name,omitempty"`
	Content   string             `json:"content,omitempty" bson:"content,omitempty"`
	FileType  string             `json:"filetype,omitempty" bson:"filetype,omitempty"`
	ExpiresAt primitive.DateTime `json:"expiresAt,omitempty" bson:"expiresAt,omitempty"`
	AccessKey string             `json:"accessKey,omitempty" bson:"accessKey,omitempty"`
}

var charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(n int) string {
	rand.Seed(time.Now().UnixNano())
	sb := strings.Builder{}
	sb.Grow(n)
	for i := 0; i < n; i++ {
		sb.WriteByte(charset[rand.Intn(len(charset))])
	}
	return sb.String()
}

func (p *Paste) CreatePaste(src *PasteBody) error {
	if src == nil {
		return errors.New("No paste information given")
	}

	if src.Content == "" {
		return errors.New("Content field empty")
	}
	p.Content = src.Content

	// Optional fields
	if src.Name != "" {
		p.Name = src.Name
	}

	if src.FileType != "" {
		p.FileType = src.FileType
	}

	// Default expiration time to 14 days if not set or set outside
	// the valid range of 1-30 days
	days := 14
	if src.ExpiresIn > 0 && src.ExpiresIn <= 30 {
		days = src.ExpiresIn
	}
	now := time.Now()
	twoWeeks := time.Hour * 24 * time.Duration(days)
	diff := now.Add(twoWeeks)
	p.ExpiresAt = primitive.NewDateTimeFromTime(diff)

	p.UUID = uuid.New().String()
	p.AccessKey = randomString(25)

	return nil
}

func toBsonDoc(p *Paste) (bson.D, error) {
	var doc bson.D
	data, err := bson.Marshal(p)
	if err != nil {
		return nil, err
	}

	err = bson.Unmarshal(data, &doc)
	return doc, err
}

func bsonToPaste(b bson.M) (paste Paste, err error) {
	doc, err := bson.Marshal(b)
	if err != nil {
		return paste, errors.New("Error marshalling BSON document")
	}
	err = bson.Unmarshal(doc, &paste)
	if err != nil {
		return paste, errors.New("Error unmarshalling BSON byte stream")
	}

	return paste, err
}

/* POST /
r.Body:
	"content"  -> required
	"name"     -> optional
	"filetype" -> optional
	"expires"  -> optional (NUMBER OF DAYS)

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

		var paste Paste
		var body PasteBody
		if err := decodeJSONBody(w, r, &body); err != nil {
			var mr *malformedRequest
			if errors.As(err, &mr) {
				http.Error(w, mr.msg, mr.status)
			} else {
				log.Print(err.Error())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		paste.CreatePaste(&body)
		doc, err := toBsonDoc(&paste)
		if err != nil {
			log.Println(err)
			http.Error(w, "Error converting request body to BSON document", http.StatusInternalServerError)
		}

		// Create document
		coll := s.Client.Database("pastes").Collection("files")
		_, err = coll.InsertOne(context.Background(), doc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := make(map[string]string)
		response["uuid"] = paste.UUID
		response["accessKey"] = paste.AccessKey
		response["expiresAt"] = paste.ExpiresAt.Time().Format(time.UnixDate)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
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
		project := bson.D{
			{Key: "_id", Value: 0},
			{Key: "accessKey", Value: 0},
			{Key: "uuid", Value: 0},
		}

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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Convert to long date format
		result["expiresAt"] = primitive.DateTime(result["expiresAt"].(primitive.DateTime)).Time().String()

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
	"expiresIn"   -> optional
	^ At least one of the 4 optional fields must be updated

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

/* Properly handle JSON request body
Helper function to decode JSON body into given struct and handle errors
This will give detailed error messages and the relevent statusCodes
regarding the error given as by default the error messages expose too
much information which is not very useful for the client
*/
type malformedRequest struct {
	status int
	msg    string
}

func (mr *malformedRequest) Error() string {
	return mr.msg
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	if r.Header.Get("Content-Type") != "" {
		value, _ := header.ParseValueAndParams(r.Header, "Content-Type")
		if value != "application/json" {
			msg := "Content-Type header is not application/json"
			return &malformedRequest{status: http.StatusUnsupportedMediaType, msg: msg}
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return &malformedRequest{status: http.StatusBadRequest, msg: msg}

		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := fmt.Sprintf("Request body contains badly-formed JSON")
			return &malformedRequest{status: http.StatusBadRequest, msg: msg}

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return &malformedRequest{status: http.StatusBadRequest, msg: msg}

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return &malformedRequest{status: http.StatusBadRequest, msg: msg}

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			return &malformedRequest{status: http.StatusBadRequest, msg: msg}

		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 1MB"
			return &malformedRequest{status: http.StatusRequestEntityTooLarge, msg: msg}

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		return &malformedRequest{status: http.StatusBadRequest, msg: msg}
	}

	return nil
}
