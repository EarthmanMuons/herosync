package youtube

// https://github.com/googleapis/google-api-go-client
// https://developers.google.com/youtube/v3/docs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const redirectURL = "http://127.0.0.1:8090" // localhost loopback address

// GetClient creates an HTTP client using OAuth2 with the given scope.
// It reads client_secret.json, and if no cached token exists,
// it launches a web server for the OAuth2 flow.
func GetClient(ctx context.Context, file string, scopes []string) *http.Client {
	jsonKey, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(jsonKey, scopes...)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	config.RedirectURL = redirectURL

	cacheFile := filepath.Join(path.Dir(file), "token.json")
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file: %v", err)
	}

	tok, err := tokenFromFile(cacheFile)
	if err != nil {
		authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
		fmt.Println("Opening browser for authorization...")
		tok, err = getTokenFromWeb(config, authURL)
		if err != nil {
			log.Fatalf("Error retrieving token from web: %v", err)
		}
		saveToken(cacheFile, tok)
	}

	return config.Client(ctx, tok)
}

// startWebServer starts a web server on localhost:8090 to capture the auth code.
func startWebServer() (chan string, error) {
	listener, err := net.Listen("tcp", "localhost:8090")
	if err != nil {
		return nil, err
	}
	codeCh := make(chan string)
	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		codeCh <- code // send code back to the OAuth flow
		listener.Close()
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Received code: %v\nYou can now safely close this browser window.", code)
	}))
	return codeCh, nil
}

// openURL opens the provided URL in the default browser.
func openURL(urlStr string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", urlStr).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr).Start()
	case "darwin":
		return exec.Command("open", urlStr).Start()
	default:
		return fmt.Errorf("cannot open URL %s on this platform", urlStr)
	}
}

// exchangeToken exchanges the authorization code for an OAuth2 token.
func exchangeToken(config *oauth2.Config, code string) (*oauth2.Token, error) {
	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token: %v", err)
	}
	return tok, nil
}

// getTokenFromWeb launches the auth URL in a browser and waits for the token via a local web server.
func getTokenFromWeb(config *oauth2.Config, authURL string) (*oauth2.Token, error) {
	codeCh, err := startWebServer()
	if err != nil {
		return nil, fmt.Errorf("unable to start web server: %v", err)
	}

	if err := openURL(authURL); err != nil {
		return nil, fmt.Errorf("unable to open authorization URL in browser: %v", err)
	}
	fmt.Println("Your browser has been opened to an authorization URL. Waiting for authorization...")

	code := <-codeCh
	return exchangeToken(config, code)
}

// tokenFromFile retrieves a Token from a file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// saveToken saves the token to a file.
func saveToken(file string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", file)
	data, err := json.Marshal(token)
	if err != nil {
		log.Printf("Unable to marshal token: %v", err)
		return
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		log.Printf("Unable to cache oauth token: %v", err)
	}
}
