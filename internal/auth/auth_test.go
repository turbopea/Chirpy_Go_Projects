package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

/* func TestHashPassword(t *testing.T) {
	password := "eriks"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal("Failed to hash the password.")
	}
	t.Logf("Password hashing works HASH: %v\n for password: %v", hash, password)

}

func TestCheckPasswordHash(t *testing.T) {
	password := "test"
	if password == "" {
		t.Fatal("Password cannot be empty")
	}
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal("Failed to hash the password. ")

	}
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		t.Fatal("Password and Hash is not matching")
	}
	if match {
		t.Logf("Password and hash match. Password: %v \n Hash: %v", password, hash)
	}
} */

/*
	 func TestJWT(t *testing.T) {
		tests := []struct {
			userID      uuid.UUID
			tokenSecret string
			expiresIn   time.Duration
			wantErr     bool
		}{
			{userID: uuid.New(), tokenSecret: "zirnis", expiresIn: time.Hour, wantErr: false},
		}
		for _, tt := range tests {
			t.Run(tt.userID.String(), func(t *testing.T) {
				signedToken, err := MakeJWT(tt.userID, tt.tokenSecret, tt.expiresIn)
				if err != nil {
					log.Fatal("Failed to create signed token")
				}
				t.Logf("This is signed token: %v", signedToken)

			})
		}

}
*/
func TestValidateJWT(t *testing.T) {
	/* userID := uuid.New()
	JWTtoken, err := MakeJWT(userID, "zirnis", time.Hour)
	if err != nil {
		t.Fatal("Couldn't create Json Web Token")

	}
	t.Logf("Here is JWTToken: %v \n user_id: %v", JWTtoken, userID)

	userid, err := ValidateJWT(JWTtoken, "zirnis")
	if err != nil {
		t.Fatal("Wrong token or token secret is given")
	}
	t.Logf("Validation went smoothly, user_id : %v", userid)
	*/

	type testCase struct {
		nameOfCase          string
		inputTokenSecret    string
		validateTokenSecret string
		inputExpiresIn      time.Duration
		expectedError       bool
	}
	testcases := []testCase{
		{"valid JWT", "zirnis", "zirnis", time.Hour, false},
		{"Expired token", "zirnis", "zirnis", -1 * time.Hour, true},
		{"Not valid JWT or token secret", "zirnis", "Jager", time.Hour, true},
	}
	for _, tt := range testcases {
		t.Run(tt.nameOfCase, func(t *testing.T) {
			userID := uuid.New()
			token, err := MakeJWT(userID, tt.inputTokenSecret, tt.inputExpiresIn)
			if err != nil {
				t.Fatal("Failed to create JSON WEB TOKEN")
			}
			userid, err := ValidateJWT(token, tt.validateTokenSecret)
			if tt.expectedError {
				if err == nil {
					t.Fatal("Expected to fail")
				}
				return
			}
			if err != nil {
				t.Fatal("Expected validation to succeed")
			}

			t.Logf("Token succesfully validated. here is user_id: %v", userid)
		})
	}

}

func TestBearerToken(t *testing.T) {
	type testCase struct {
		name       string
		headers    http.Header
		wantString string
		wantError  bool
	}
	headers := http.Header{}
	headers.Set("Authorization", "bobber123")
	emptyHeaders := http.Header{}
	testcases := []testCase{
		{"valid header", headers, "bobber123", false},
		{"mising header", emptyHeaders, "bobber123", true},
	}
	for _, tt := range testcases {
		t.Run(tt.name, func(f *testing.T) {
			token, err := GetBearerToken(tt.headers)
			if tt.wantError {
				if err == nil {
					t.Fatal("Expected to fail")
					return
				}

			}
			t.Log(token)

		})
	}
}

func TestMakeRefreshToken(t *testing.T) {
	t.Logf("Refresh Token: %v", MakeRefreshToken())
}
