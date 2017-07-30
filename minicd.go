package minicd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pkg/errors"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"

	"github.com/google/go-github/github"
)

var errNoPushevent = errors.New("not a push event")

// Config is the struct where you specify all the values needed for minicd to deliver your app.
type Config struct {
	WebhookSecret string
	GithubToken   string
	KillSig       chan context.Context
}

// Handler is a function that returns an net/http Handler. This is the Github WebHook Handler
//  that will listen for pushes on master to build and deploy your application.
func Handler(c Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		cloneURL, headCommit, err := parseRequest(r, c.WebhookSecret)
		if err != nil {
			if err == errNoPushevent {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, err)
			return
		}

		tempPath, err := cloneRepo(c.GithubToken, cloneURL, headCommit)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}
		defer os.RemoveAll(tempPath)

		err = compilePkg(tempPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		binPath := filepath.Join(tempPath, "minicdbin")
		dstPath, _ := os.Getwd()
		err = cp(binPath, dstPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		sendKill(context.Background(), c.KillSig)

		err = run(filepath.Join(dstPath, "minicdbin"))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func sendKill(ctx context.Context, ch chan context.Context) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	ch <- ctx
	<-ctx.Done()
}

func parseRequest(r *http.Request, secret string) (cloneURL, headCommit string, err error) {
	payload, err := github.ValidatePayload(r, []byte(secret))
	if err != nil {
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		return
	}

	gpe, ok := event.(*github.PushEvent)
	if !ok {
		return "", "", errNoPushevent
	}

	if !pushValid(gpe) {
		return "", "", errors.New("invalid push")
	}

	return gpe.Repo.GetCloneURL(), gpe.HeadCommit.GetID(), nil
}

func cloneRepo(githubToken, cloneURL, headCommit string) (tempPath string, err error) {
	gitURL, err := url.Parse(cloneURL)
	if err != nil {
		return "", errors.Wrap(err, "invalid clone url")
	}

	// the reason we clone into GOPATH/src instead of ioutil.Tempdir is so that the vendor package can be read.
	tempPath = filepath.Join(getGopath(), "src", fmt.Sprintf("minicd-%v", headCommit))

	gitURL.User = url.UserPassword(githubToken, "x-oauth-basic")
	repo, err := git.PlainClone(tempPath, false, &git.CloneOptions{
		URL:               gitURL.String(),
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		ReferenceName:     plumbing.ReferenceName("refs/heads/master"),
	})

	if err != nil {
		return "", errors.Wrap(err, "could not clone repo")
	}

	wt, err := repo.Worktree()
	if err != nil {
		return "", errors.Wrap(err, "could not get Worktree")
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(headCommit),
	})

	if err != nil {
		return "", errors.Wrap(err, "could not checkout head commit")
	}

	return tempPath, nil
}

func compilePkg(path string) error {
	cmd := exec.Command("go", "build", "-o", "minicdbin")
	cmd.Dir = path
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "could not build Go binary")
	}

	return nil
}

func cp(path, dest string) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.Wrap(err, "could not open built binary")
	}
	defer f.Close()

	_, fileName := filepath.Split(path)

	fullDstPath := filepath.Join(dest, fileName)
	fdest, err := os.Create(fullDstPath)
	if err != nil {
		return errors.Wrap(err, "could not create destination binary")
	}
	defer fdest.Close()

	_, err = io.Copy(fdest, f)
	if err != nil {
		err = errors.Wrap(err, "could not copy binary to destination")
	}
	fdest.Close()

	err = os.Chmod(fullDstPath, 0555)
	if err != nil {
		err = errors.Wrap(err, "could not make new binary executable")
	}

	return err
}

func run(binPath string) error {
	cmd := exec.Command(binPath)

	err := cmd.Start()

	return err
}

func pushValid(pe *github.PushEvent) bool {
	return !pe.GetDeleted() && pe.HeadCommit != nil // add the infinite loop check.
}

func getGopath() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = defaultGOPATH()
	}

	return gopath
}

func defaultGOPATH() string {
	env := "HOME"
	if runtime.GOOS == "windows" {
		env = "USERPROFILE"
	} else if runtime.GOOS == "plan9" {
		env = "home"
	}
	if home := os.Getenv(env); home != "" {
		def := filepath.Join(home, "go")
		if filepath.Clean(def) == filepath.Clean(runtime.GOROOT()) {
			// Don't set the default GOPATH to GOROOT,
			// as that will trigger warnings from the go tool.
			return ""
		}
		return def
	}
	return ""
}
