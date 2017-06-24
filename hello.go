package guestbook

import (
	"html/template"
	"net/http"
	"time"

	"fmt"
	"github.com/stretchr/gomniauth"
	"github.com/stretchr/gomniauth/common"
	"github.com/stretchr/gomniauth/providers/google"
	"github.com/stretchr/objx"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
	"google.golang.org/appengine/user"
	"io"
	"os"
)

type Greeting struct {
	Author  string
	Content string
	Date    time.Time
}

type OAuthToken struct {
	Name         string
	ClientID     string
	ClientSecret string
}

func init() {
	http.HandleFunc("/", root)
	http.HandleFunc("/sign", sign)
	http.HandleFunc("/oauth/google/login", oauthGoogleLogin)
	http.HandleFunc("/oauth/google/callback", oauthGoogleCallback)
}

func fetchGoogleOAuthKey(ctx context.Context) (*OAuthToken, error) {
	key := datastore.NewKey(ctx, "OAuthToken", "google", 0, nil)
	oauthToken := &OAuthToken{}

	if err := datastore.Get(ctx, key, oauthToken); err == datastore.ErrNoSuchEntity {
		log.Infof(ctx, "Not such entity.. %#v", key)
		oauthToken.Name = "default"
		oauthToken.ClientID = "<ClientID>"
		oauthToken.ClientSecret = "<ClientSecret>"
		if _, err := datastore.Put(ctx, key, oauthToken); err != nil {
			return oauthToken, err
		}
	} else if err != nil {
		return oauthToken, err
	}

	return oauthToken, nil
}

func initializeGoogleLogin(ctx context.Context, w http.ResponseWriter, r *http.Request) (*google.GoogleProvider, error) {
	oauthToken, err := fetchGoogleOAuthKey(ctx)
	if err != nil {
		return nil, err
	}

	gomniauth.SetSecurityKey(os.Getenv("GOMNIAUTH_SECRET"))

	googleProvider := google.New(
		oauthToken.ClientID,
		oauthToken.ClientSecret,
		getAccessURL(ctx, r)+"/oauth/google/callback",
	)

	gomniauth.WithProviders(
		googleProvider,
	)

	t := new(urlfetch.Transport)
	t.Context = ctx
	common.SetRoundTripper(t)

	return googleProvider, nil
}

func oauthGoogleLogin(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	provider, err := initializeGoogleLogin(ctx, w, r)
	if err != nil {
		panic(err)
	}

	state := gomniauth.NewState("after", "success")
	authUrl, err := provider.GetBeginAuthURL(state, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, authUrl, http.StatusFound)

	// fmt.Fprint(w, "Hi, Login Page")
}

func oauthGoogleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	provider, err := initializeGoogleLogin(ctx, w, r)
	if err != nil {
		panic(err)
	}
	omap, err := objx.FromURLQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	creds, err := provider.CompleteAuth(omap)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// load the user
	user, userErr := provider.GetUser(creds)

	if userErr != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := fmt.Sprintf("%#v", user)
	io.WriteString(w, data)
}

func getAccessURL(ctx context.Context, r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if r.Header.Get("X-Forwarded-Scheme") == "https" {
		scheme = "https"
	}

	hostname := r.Host
	port := r.Header.Get("X-Server-Port")
	if port != "" {
		hostname = fmt.Sprintf("%s:%s", hostname, port)
	}

	url := fmt.Sprintf("%s://%s", scheme, hostname)

	return url
}

// guestbookKey returns the key used for all guestbook entries.
func guestbookKey(c context.Context) *datastore.Key {
	// The string "default_guestbook" here could be varied to have multiple guestbooks.
	return datastore.NewKey(c, "Guestbook", "default_guestbook", 0, nil)
}

// [START func_root]
func root(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	// Ancestor queries, as shown here, are strongly consistent with the High
	// Replication Datastore. Queries that span entity groups are eventually
	// consistent. If we omitted the .Ancestor from this query there would be
	// a slight chance that Greeting that had just been written would not
	// show up in a query.
	// [START query]
	q := datastore.NewQuery("Greeting").Ancestor(guestbookKey(c)).Order("-Date").Limit(10)
	// [END query]
	// [START getall]
	greetings := make([]Greeting, 0, 10)
	if _, err := q.GetAll(c, &greetings); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// [END getall]
	if err := guestbookTemplate.Execute(w, greetings); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// [END func_root]

var guestbookTemplate = template.Must(template.New("book").Parse(`
<html>
  <head>
    <title>Go Guestbook</title>
  </head>
  <body>
  	<a href="/oauth/google/login">Google Login</a>
    {{range .}}
      {{with .Author}}
        <p><b>{{.}}</b> wrote:</p>
      {{else}}
        <p>An anonymous person wrote:</p>
      {{end}}
      <pre>{{.Content}}</pre>
    {{end}}
    <form action="/sign" method="post">
      <div><textarea name="content" rows="3" cols="60"></textarea></div>
      <div><input type="submit" value="Sign Guestbook"></div>
    </form>
  </body>
</html>
`))

// [START func_sign]
func sign(w http.ResponseWriter, r *http.Request) {
	// [START new_context]
	c := appengine.NewContext(r)
	// [END new_context]
	g := Greeting{
		Content: r.FormValue("content"),
		Date:    time.Now(),
	}
	// [START if_user]
	if u := user.Current(c); u != nil {
		g.Author = u.String()
	}
	// We set the same parent key on every Greeting entity to ensure each Greeting
	// is in the same entity group. Queries across the single entity group
	// will be consistent. However, the write rate to a single entity group
	// should be limited to ~1/second.
	key := datastore.NewIncompleteKey(c, "Greeting", guestbookKey(c))
	_, err := datastore.Put(c, key, &g)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
	// [END if_user]
}
