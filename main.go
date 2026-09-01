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
	polkakey       string
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	polkakey := os.Getenv("POLKA_KEY")
	if polkakey == "" {
		log.Fatal("Invalid API KEY")
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
		polkakey:       polkakey,
	}
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))
	mux.HandleFunc("GET /admin/metrics", apiCfg.metrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.reset)
	mux.HandleFunc("POST /api/users", apiCfg.handlerUsersCreate)
	mux.HandleFunc("POST /api/chirps", apiCfg.MessageChirps)
	mux.HandleFunc("GET /api/chirps/", apiCfg.GetAllChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.GetSingleChirp)
	mux.HandleFunc("POST /api/login", apiCfg.UserLogin)
	mux.HandleFunc("POST /api/refresh", apiCfg.Refresh)
	mux.HandleFunc("POST /api/revoke", apiCfg.Revoke)
	mux.HandleFunc("PUT /api/users", apiCfg.ModifyCredentials)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.DeleteChirp)
	mux.HandleFunc("POST /api/polka/webhooks", apiCfg.Webhooks)
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
	ID            uuid.UUID `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Email         string    `json:"email"`
	Token         string    `json:"token"`
	Refresh_token string    `json:"refresh_token"`
	Is_chirpy_red bool      `json:"is_chirpy_red"`
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
			ID:            user.ID,
			CreatedAt:     user.CreatedAt,
			UpdatedAt:     user.UpdatedAt,
			Email:         user.Email,
			Is_chirpy_red: user.IsChirpyRed,
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

	authordIDQUERY := r.URL.Query().Get("author_id")
	if authordIDQUERY != "" {
		uuidAuthorID, err := uuid.Parse(authordIDQUERY)
		if err != nil {
			respondWithError(w, http.StatusConflict, "Failed to get from uuid string to uuid.UUID")
			return
		}
		allUserChirps, err := cfg.db.GetAllUserChirpsByID(r.Context(), uuidAuthorID)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Author ID wrong")
			return

		}
		respondWithJSON(w, http.StatusOK, allUserChirps)
	}
	sort := r.URL.Query().Get("sort")

	if "asc" == sort {
		sortQuery, err := cfg.db.GetAllUserChirps(r.Context())
		if err != nil {
			respondWithError(w, http.StatusConflict, "Failed to sort in ascending order")
			return
		}
		respondWithJSON(w, http.StatusOK, sortQuery)

	} else if "desc" == sort {
		sortQuery, err := cfg.db.GetAllUserChirpsDESC(r.Context())
		if err != nil {
			respondWithError(w, http.StatusConflict, "Failed to sort in descending order")
			return
		}
		respondWithJSON(w, http.StatusOK, sortQuery)
	} else {
		sortQuery, err := cfg.db.GetAllUserChirps(r.Context())
		if err != nil {
			respondWithError(w, http.StatusConflict, "Failed to sort in ascending order")
			return
		}
		respondWithJSON(w, http.StatusOK, sortQuery)

	}

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
		Password string `json:"password"`
		Email    string `json:"email"`
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
	passwordMatch, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}
	if passwordMatch != true {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}
	jwttoken, err := auth.MakeJWT(user.ID, os.Getenv("TOKEN_SECRET"), time.Hour)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to create JSON WEB TOKEN")
		return
	}
	refreshToken := auth.MakeRefreshToken()
	addToDBToken, err := cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:  refreshToken,
		UserID: user.ID,
	})
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to Create Refresh Token")
		return
	}

	if passwordMatch {
		respondWithJSON(w, http.StatusOK, response{
			User: User{
				ID:            user.ID,
				CreatedAt:     user.CreatedAt,
				UpdatedAt:     user.UpdatedAt,
				Email:         user.Email,
				Token:         jwttoken,
				Refresh_token: addToDBToken.Token,
				Is_chirpy_red: user.IsChirpyRed,
			},
		})
	}

}

func (cfg *apiConfig) Refresh(w http.ResponseWriter, r *http.Request) {
	type response struct {
		Token string `json:"token"`
	}
	RefreshToken, err := auth.GetBearerToken(r.Header)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to Get Refresh Token")
		return
	}
	checkToken, err := cfg.db.CheckIfTokenExists(r.Context(), RefreshToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Refresh token doesn't exist, it is expired or revoked.")
		return
	}
	getUserID, err := cfg.db.GetUserFromRefreshToken(r.Context(), checkToken.Token)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to get UserID")
		return
	}
	MakeJWT, err := auth.MakeJWT(getUserID, os.Getenv("TOKEN_SECRET"), time.Hour)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to create JWT token.")
		return

	}

	respondWithJSON(w, http.StatusOK, response{Token: MakeJWT})
}

func (cfg *apiConfig) Revoke(w http.ResponseWriter, r *http.Request) {
	RefreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to Get Refresh Token")
		return
	}
	tokenInDB, err := cfg.db.CheckIfTokenExists(r.Context(), RefreshToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Refresh token doesn't exist, it is expired or revoked.")
		return
	}
	err = cfg.db.RevokeToken(r.Context(), tokenInDB.Token)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error revoking token")
	}
	w.WriteHeader(http.StatusNoContent)

}

func (cfg *apiConfig) ModifyCredentials(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type response struct {
		User
	}
	params := parameters{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to Decode parameters")
		return
	}
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Failed to get token from header")
		return
	}
	userid, err := auth.ValidateJWT(token, os.Getenv("TOKEN_SECRET"))
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "User doesn't exist")
		return
	}
	HashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Failed to Hash password")
		return
	}
	CheckIfUserExists, err := cfg.db.CheckIfUserExistByID(r.Context(), userid)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "User ID doesn't exis")
		return
	}
	if userid == CheckIfUserExists {
		accessUserTable, err := cfg.db.UpdateUser(r.Context(), database.UpdateUserParams{
			Email:          params.Email,
			HashedPassword: HashedPassword,
		})
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Failed to update password and email.")
			return
		}
		respondWithJSON(w, http.StatusOK, response{
			User: User{
				Email: accessUserTable.Email,
			},
		})
	}

}

func (cfg *apiConfig) DeleteChirp(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Header doesn't contain token")
		return
	}
	userid, err := auth.ValidateJWT(token, os.Getenv("TOKEN_SECRET"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Such user doesn't exist")
		return
	}
	MessageID := r.PathValue("chirpID")
	ParsedMessageID, err := uuid.Parse(MessageID)

	if err != nil {
		log.Println(MessageID)
		log.Println(ParsedMessageID)
		respondWithError(w, http.StatusForbidden, ParsedMessageID.String())
		return
	}
	GetSingleChirp, err := cfg.db.GetSingleChirp(r.Context(), ParsedMessageID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Chirp doesn't exist")
		return
	}
	if GetSingleChirp.UserID == userid {
		cfg.db.DelChirp(r.Context(), GetSingleChirp.MessageID)
		w.WriteHeader(http.StatusNoContent)
		return
	} else {
		w.WriteHeader(http.StatusForbidden)
		return
	}

}

func (cfg *apiConfig) Webhooks(w http.ResponseWriter, r *http.Request) {
	type Data struct {
		User_id uuid.UUID `json:"user_id"`
	}
	type parameters struct {
		Event string `json:"event"`
		Data  Data   `json:"data"`
	}
	params := parameters{}
	decode := json.NewDecoder(r.Body)
	err := decode.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to decode the params")

	}
	apiKey, err := auth.GetAPIKEY(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Error")
		return
	}
	if apiKey != os.Getenv("POLKA_KEY") {
		w.WriteHeader(http.StatusUnauthorized)
	}

	if params.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
	} else if params.Event == "user.upgraded" {
		updateUser, err := cfg.db.UpgradeUserByID(r.Context(), params.Data.User_id)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Couldn't update the user")
			return
		}
		updateUser.IsChirpyRed = true
		w.WriteHeader(http.StatusNoContent)

	}

}
