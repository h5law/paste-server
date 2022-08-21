/*
Copyright © 2022 Harry Law <hrryslw@pm.me>
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice,
   this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its contributors
   may be used to endorse or promote products derived from this software
   without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.
*/
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/golang/gddo/httputil/header"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/h5law/paste-server/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	dbName   string = "pastes"
	collName string = "files"
)

type Handler struct {
	*mux.Router
	*mongo.Client
}

func (h *Handler) ConnectDB(uri string) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		log.Print("fatal", "%v", err)
	}
	if err := client.Ping(nil, nil); err != nil {
		log.Print("fatal", "failed to connect to database: %v", err)
	}
	h.Client = client
	log.Print("info", "connected to database")
}

func (h *Handler) DisconnectDB() {
	if err := h.Client.Disconnect(context.Background()); err != nil {
		log.Print("fatal", "failed to disconnect from database: %v", err)
	}
	h.Client = nil
	log.Print("info", "disconnected from database")
}

func (h *Handler) routes() {
	h.HandleFunc("/", h.createPaste()).Methods("POST")
	h.HandleFunc("/{uuid}", h.getPaste()).Methods("GET")
	h.HandleFunc("/{uuid}", h.updatePaste()).Methods("PUT")
	h.HandleFunc("/{uuid}", h.deletePaste()).Methods("DELETE")
}

func NewHandler() *Handler {
	h := &Handler{
		Router: mux.NewRouter(),
	}

	h.routes()
	return h
}

type PasteBody struct {
	Content   []string `json:"content"`
	FileType  string   `json:"filetype,omitempty"`
	ExpiresIn int      `json:"expiresIn,omitempty"`
	AccessKey string   `json:"accessKey,omitempty"`
}

type Paste struct {
	UUID      string             `json:"uuid,omitempty" bson:"uuid,omitempty"`
	Content   []string           `json:"content,omitempty" bson:"content,omitempty"`
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

func (p *Paste) NewPaste(src *PasteBody) error {
	if src == nil {
		return errors.New("No paste information given")
	}

	if src.Content == nil {
		return errors.New("Content field empty")
	}
	p.Content = src.Content

	// Default to plaintext if not set
	p.FileType = "plaintext"
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

func (p *Paste) EditPaste(src *PasteBody) error {
	if src == nil {
		return errors.New("No updates given")
	}

	// Check if any changes have been made and are valid
	if src.Content != nil && reflect.DeepEqual(src.Content, p.Content) {
		return errors.New("No changes made to content field")
	}
	if src.FileType != "" && src.FileType == p.FileType {
		return errors.New("No changes made to filetype field")
	}
	if src.ExpiresIn != 0 && src.ExpiresIn <= 0 || src.ExpiresIn >= 30 {
		return errors.New("Expiration time outside valid range")
	}

	if src.AccessKey != "" {
		p.AccessKey = src.AccessKey
	}

	// Apply changes
	if src.Content != nil {
		p.Content = src.Content
	}
	if src.FileType != "" {
		p.FileType = src.FileType
	}

	// Set new expiration date defaulting to 14 days
	days := 14
	if src.ExpiresIn > 0 && src.ExpiresIn <= 30 {
		days = src.ExpiresIn
	}
	now := time.Now()
	twoWeeks := time.Hour * 24 * time.Duration(days)
	diff := now.Add(twoWeeks)
	p.ExpiresAt = primitive.NewDateTimeFromTime(diff)

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

func bsonToPaste(b bson.M) (Paste, error) {
	var paste Paste
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
	"content"   -> required
	"filetype"  -> optional
	"expiresIn" -> optional (NUMBER OF DAYS)

Creates a new Paste in the MongoDB database and returns a JSON document
{
	uuid:		UUID,
	content:	[]String,
	filetype:	String,
	accessKey:  String,
	expires:	Date
}
*/
func (h *Handler) createPaste() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			log.Print("info", "%s %s [%v]",
				r.Method,
				r.URL.Path,
				time.Since(start),
			)
		}()

		// Load body into struct
		var paste Paste
		var body PasteBody
		if err := decodeJSONBody(w, r, &body); err != nil {
			var mr *badRequest
			if errors.As(err, &mr) {
				http.Error(w, mr.msg, mr.status)
			} else {
				log.Print("error", "%v", err.Error())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		// Create new Paste struct and convert into a BSON document
		if err := paste.NewPaste(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		doc, err := toBsonDoc(&paste)
		if err != nil {
			log.Print("error", "%v", err)
			http.Error(w, "Error converting request body to BSON document", http.StatusInternalServerError)
		}

		// Create document
		coll := h.Client.Database(dbName).Collection(collName)
		_, err = coll.InsertOne(context.TODO(), doc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := make(map[string]string)
		response["uuid"] = paste.UUID
		response["accessKey"] = paste.AccessKey
		response["expiresAt"] = paste.ExpiresAt.Time().String()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
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
	content:	[]String,
	filetype:	String,
	accessKey:  String,
	expires:	Date
}
*/
func (h *Handler) getPaste() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			log.Print("info", "%s %s [%v]",
				r.Method,
				r.URL.Path,
				time.Since(start),
			)
		}()

		uuidStr, _ := mux.Vars(r)["uuid"]

		// Fetch document matching UUID from database
		coll := h.Client.Database(dbName).Collection(collName)
		var result bson.M
		filter := bson.M{"uuid": uuidStr}
		project := bson.M{
			"_id":       0,
			"accessKey": 0,
			"uuid":      0,
		}

		err := coll.FindOne(
			context.TODO(),
			filter,
			options.FindOne().SetProjection(project),
		).Decode(&result)

		if err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, "No document found with that UUID", http.StatusBadRequest)
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
	"accessKey"   -> required
	"content"	  -> optional
	"filetype"    -> optional
	"expiresIn"   -> optional
	^ At least one of the 3 optional fields must be updated

Updates an existing Paste in the MongoDB database and returns a JSON document
{
	uuid:		UUID,
	expiresAt:	Date
}
*/
func (h *Handler) updatePaste() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			log.Print("info", "%s %s [%v]",
				r.Method,
				r.URL.Path,
				time.Since(start),
			)
		}()

		uuidStr, _ := mux.Vars(r)["uuid"]

		// Load body into struct
		var body PasteBody
		if err := decodeJSONBody(w, r, &body); err != nil {
			var mr *badRequest
			if errors.As(err, &mr) {
				http.Error(w, mr.msg, mr.status)
			} else {
				log.Print("error", "%v", err.Error())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		// Get and load current document state
		coll := h.Client.Database(dbName).Collection(collName)
		var result bson.M
		filter := bson.M{"uuid": uuidStr}
		project := bson.M{
			"_id": 0,
		}

		err := coll.FindOne(
			context.TODO(),
			filter,
			options.FindOne().SetProjection(project),
		).Decode(&result)

		if err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, "No document found with that UUID", http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Convert BSON result to Paste struct
		paste, err := bsonToPaste(result)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Update Paste and check for errors
		if err := paste.EditPaste(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Convert updated paste to BSON document
		doc, err := toBsonDoc(&paste)
		if err != nil {
			log.Print("error", "%v", err)
			http.Error(w, "Error converting request body to BSON document", http.StatusInternalServerError)
		}

		// Check the sender can actually edit the paste
		if paste.AccessKey != result["accessKey"] {
			http.Error(w, "Invalid access key", http.StatusUnauthorized)
			return
		}

		// Update document
		filter = bson.M{"uuid": uuidStr}
		update := bson.M{"$set": doc}
		res, err := coll.UpdateOne(context.TODO(), filter, update)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if res.MatchedCount == 0 || res.ModifiedCount == 0 {
			http.Error(w, "Error matching and updating document", http.StatusInternalServerError)
			return
		}

		response := make(map[string]string)
		response["uuid"] = uuidStr
		response["expiresAt"] = paste.ExpiresAt.Time().String()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

/* DELETE /{uuid}
r.Body:
	"accessKey"  -> required

Deletes an existing Paste in the MongoDB database
*/
func (h *Handler) deletePaste() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			log.Print("info", "%s %s [%v]",
				r.Method,
				r.URL.Path,
				time.Since(start),
			)
		}()

		uuidStr, _ := mux.Vars(r)["uuid"]

		// Load body into struct
		body := struct {
			AccessKey string `json:"accessKey,omitempty"`
		}{}
		if err := decodeJSONBody(w, r, &body); err != nil {
			var mr *badRequest
			if errors.As(err, &mr) {
				http.Error(w, mr.msg, mr.status)
			} else {
				log.Print("error", "%v", err.Error())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		// Check document exists and accessKey is the same
		coll := h.Client.Database(dbName).Collection(collName)
		var result bson.M
		filter := bson.M{"uuid": uuidStr}
		project := bson.M{
			"_id":       0,
			"accessKey": 1,
		}

		err := coll.FindOne(
			context.TODO(),
			filter,
			options.FindOne().SetProjection(project),
		).Decode(&result)

		if err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, "No document found with that UUID", http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Check the sender can actually edit the paste
		if body.AccessKey != result["accessKey"] {
			http.Error(w, "Invalid access key", http.StatusUnauthorized)
			return
		}

		// Delete matching document
		//opts := options.Delete().SetHint(bson.D{{Key: "uuid", Value: 1}})
		res, err := coll.DeleteOne(context.TODO(), filter, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if res.DeletedCount == 0 {
			http.Error(w, "Error matching and deleting document", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

/* Properly handle JSON request body
Helper function to decode JSON body into given struct and handle errors
This will give detailed error messages and the relevent statusCodes
regarding the error given as by default the error messages expose too
much information which is not very useful for the client
*/
type badRequest struct {
	status int
	msg    string
}

func (mr *badRequest) Error() string {
	return mr.msg
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	// Check header is JSON
	if r.Header.Get("Content-Type") != "" {
		value, _ := header.ParseValueAndParams(r.Header, "Content-Type")
		if value != "application/json" {
			msg := "Content-Type header is not application/json"
			return &badRequest{status: http.StatusUnsupportedMediaType, msg: msg}
		}
	}

	// Set max body size to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	// Create decoder and throw error with non recognised fields
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&dst); err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return &badRequest{status: http.StatusBadRequest, msg: msg}

		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := fmt.Sprintf("Request body contains badly-formed JSON")
			return &badRequest{status: http.StatusBadRequest, msg: msg}

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return &badRequest{status: http.StatusBadRequest, msg: msg}

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return &badRequest{status: http.StatusBadRequest, msg: msg}

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			return &badRequest{status: http.StatusBadRequest, msg: msg}

		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 1MB"
			return &badRequest{status: http.StatusRequestEntityTooLarge, msg: msg}

		default:
			return err
		}
	}

	// Try and decode again to check only a single JSON object was sent
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		return &badRequest{status: http.StatusBadRequest, msg: msg}
	}

	return nil
}
