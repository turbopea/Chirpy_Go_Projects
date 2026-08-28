package main

import (
	"Chirpy_Go_Projects/Chirpy/Chirpy_Go_Projects/internal/auth"
	"Chirpy_Go_Projects/Chirpy/Chirpy_Go_Projects/internal/database"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/joho/godotenv"

	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	jwtsecret      string
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	jwtsecret := os.Getenv("TOKEN_SECRET")
	if jwtsecret == "" {
		log.Fatal("jwtsecret must be set")
	}
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	if platform == "" {
		log.Fatal("PLATFORM Must be set")
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Error opening postgre database")
	}
	dbQueries := database.New(db)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", healthCheck)
	handler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	apiCfg := &apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
		platform:       platform,
	}
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))
	mux.HandleFunc("GET /admin/metrics", apiCfg.metrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.reset)
	mux.HandleFunc("POST /api/users", apiCfg.handlerUsersCreate)
	mux.HandleFunc("POST /api/chirps", apiCfg.MessageChirps)
	mux.HandleFunc("GET /api/chirps/", apiCfg.GetAllChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.GetSingleChirp)
	mux.HandleFunc("POST /api/login", apiCfg.UserLogin)
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	log.Fatal(server.ListenAndServe())
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})

}

func (cfg *apiConfig) metrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	x := cfg.fileserverHits.Load()
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html`, x)))
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {

		w.Write([]byte(fmt.Sprintf("HTTP method %d not allowed", r.Method)))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))

}
func (cfg *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Reset is only allowed in dev environment."))
		return
	}

	cfg.fileserverHits.Store(0)
	err := cfg.db.Reset(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Failed to reset the database: " + err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Reset to 0"))
}

func validate_chirp(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	type parameters struct {
		Body string `json:"body"`
	}
	type returnVals struct {
		CleanedBody string `json:"cleaned_body"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	if len(params.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}
	badWords := badWords()
	textArray := strings.Split(params.Body, ` `)

	for _, badWord := range badWords {
		for i, word := range textArray {
			if strings.ToLower(word) == badWord {
				textArray[i] = "****"
			}

		}
	}
	joinedString := strings.Join(textArray, " ")
	respondWithJSON(w, 200, returnVals{
		CleanedBody: joinedString,
	})
}
func respondWithError(w http.ResponseWriter, code int, msg string) error {
	return respondWithJSON(w, code, map[string]string{"error": msg})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) error {
	response, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	w.Write(response)
	return nil
}

func badWords() []string {

	listofWords := []string{
		"kerfuffle",
		"sharbert",
		"fornax",
	}

	return listofWords
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
}

func (cfg *apiConfig) handlerUsersCreate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type response struct {
		User
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}

	err := decoder.Decode(&params)
	if err != nil {

		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters")
		return
	}
	HashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't hash password")
	}

	user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		HashedPassword: HashedPassword,
		Email:          params.Email,
	})
	if err != nil {
		log.Printf("CreateUser error: %v", err)
		respondWithError(w, http.StatusInternalServerError, "couldn't create user")
		return
	}
	respondWithJSON(w, http.StatusCreated, response{
		User: User{
			ID:        user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Email:     user.Email,
		},
	})
}

type Chirp struct {
	Message_id uuid.UUID `json:"id"`
	Created_at time.Time `json:"created_at"`
	Updated_at time.Time `json:"updated_at"`
	Body       string    `json:"body"`
	User_id    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) MessageChirps(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	type response struct {
		Chirp
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters")
		return
	}

	defer r.Body.Close()
	if len(params.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "failed to get token from header")
		return
	}
	user, err := auth.ValidateJWT(token, os.Getenv("TOKEN_SECRET"))
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "failed to validate token")
		return
	}

	msg, err := cfg.db.CreateMessage(r.Context(), database.CreateMessageParams{
		Body:   params.Body,
		UserID: user,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't create a message")
		return
	}
	respondWithJSON(w, http.StatusCreated, response{
		Chirp: Chirp{
			Message_id: msg.MessageID,
			Created_at: msg.CreatedAt,
			Updated_at: msg.UpdatedAt,
			Body:       msg.Body,
			User_id:    user,
		},
	})

}

func (cfg *apiConfig) GetAllChirps(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.db.GetAllChirps(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't get chirps")
		return
	}

	respondWithJSON(w, http.StatusOK, chirps)
}

func (cfg *apiConfig) GetSingleChirp(w http.ResponseWriter, r *http.Request) {
	MessageID := r.PathValue("chirpID")
	type response struct {
		Chirp
	}
	ParsedMessageID, err := uuid.Parse(MessageID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Chirp ID")
	}
	chirp, err := cfg.db.GetSingleChirp(r.Context(), ParsedMessageID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "That chirp doesn't exist")
		return
	}
	respondWithJSON(w, http.StatusOK, chirp)
}

func (cfg *apiConfig) UserLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password         string        `json:"password"`
		Email            string        `json:"email"`
		ExpiresInSeconds time.Duration `json:"expires_in_seconds"`
	}
	type response struct {
		User
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "couldn't decode params")
		return
	}
	user, err := cfg.db.GetUser(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "couldn't find a user")
		return
	}
	if params.ExpiresInSeconds == 0 {
		params.ExpiresInSeconds = time.Hour
	} else if params.ExpiresInSeconds > 3600 {
		params.ExpiresInSeconds = time.Hour
	}

	passwordMatch, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}
	if passwordMatch != true {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}
	jwttoken, err := auth.MakeJWT(user.ID, os.Getenv("TOKEN_SECRET"), params.ExpiresInSeconds)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to create JSON WEB TOKEN")
	}
	if passwordMatch {
		respondWithJSON(w, http.StatusOK, response{
			User: User{
				ID:        user.ID,
				CreatedAt: user.CreatedAt,
				UpdatedAt: user.UpdatedAt,
				Email:     user.Email,
				Token:     jwttoken,
			},
		})
	}

}
