package main

import (
	"fmt"
	"html/template"
	"log"
	"strings"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

// Repository wraps a go-git Repository object.
type Repository struct {
	R *git.Repository

	UniqueLen   int // Shortest possible but still unique commit hash length
	ShortToLong map[string]plumbing.Hash
}

type eHashTooShort struct{}

func (e eHashTooShort) Error() string {
	return ""
}

func testShortLength(r *git.Repository, sl int, branches []plumbing.Hash) map[string]plumbing.Hash {
	ret := make(map[string]plumbing.Hash)
	for _, branch := range branches {
		logIter, err := r.Log(&git.LogOptions{From: branch})
		if err != nil {
			log.Fatalln(err)
		}
		err = logIter.ForEach(func(c *object.Commit) error {
			curFull := c.Hash
			curShort := curFull.String()[:sl]
			if prevFull, ok := ret[curShort]; ok && prevFull != curFull {
				return eHashTooShort{}
			}
			ret[curShort] = curFull
			return nil
		})
		if _, ok := err.(eHashTooShort); ok {
			return nil
		}
	}
	return ret
}

// NewRepository calls optimalClone(url), and wraps the result into our
// Repository type.
func NewRepository(url string) (ret Repository) {
	r, err := optimalClone(url)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Done.")

	// Collect all branches
	var branches []plumbing.Hash
	branchIter, err := r.Branches()
	if err != nil {
		log.Fatalln(err)
	}
	if err = branchIter.ForEach(func(ref *plumbing.Reference) error {
		branches = append(branches, ref.Hash())
		return nil
	}); err != nil {
		log.Fatalln(err)
	}

	testlen := 0
	for testlen < 40 && ret.ShortToLong == nil {
		testlen++
		ret.ShortToLong = testShortLength(r, testlen, branches)
	}
	log.Printf("shortest unique hash length is %v characters", testlen)

	ret.UniqueLen = testlen
	ret.R = r
	return
}

var repo Repository
var master *plumbing.Hash

func getCommit(rev string) (*object.Commit, error) {
	hash, err := repo.R.ResolveRevision(plumbing.Revision(rev))
	if err == nil {
		return repo.R.CommitObject(*hash)
	}
	if len(rev) >= repo.UniqueLen {
		if hash, ok := repo.ShortToLong[rev[:repo.UniqueLen]]; ok {
			return repo.R.CommitObject(hash)
		}
	}
	return nil, err
}

func getLog() (object.CommitIter, error) {
	return repo.R.Log(&git.LogOptions{From: *master})
}

type commitInfo struct {
	Time time.Time
	Desc string
	Hash plumbing.Hash
}

func makeCommitInfo(c *object.Commit) commitInfo {
	desc := c.Message
	if i := strings.IndexByte(desc, '\n'); i > 0 {
		desc = desc[:i]
	}
	return commitInfo{
		Time: c.Author.When,
		Desc: desc,
		Hash: c.Hash,
	}
}

// CommitLink returns a nicely formatted link to rev in the ReC98 repository.
func CommitLink(rev string) template.HTML {
	revEsc := template.HTMLEscapeString(rev)
	return template.HTML(fmt.Sprintf(
		`<a href="https://github.com/nmlgc/ReC98/commit/%s"><code>%s</code></a>`,
		revEsc, revEsc,
	))
}

func commits(iter object.CommitIter) chan commitInfo {
	ret := make(chan commitInfo)
	go func() {
		err := iter.ForEach(func(c *object.Commit) error {
			ret <- makeCommitInfo(c)
			return nil
		})
		close(ret)
		if err != nil {
			log.Panicln(err)
		}
	}()
	return ret
}

func optimalClone(url string) (*git.Repository, error) {
	if strings.HasPrefix(url, "https://") {
		// TODO: This takes 3 GB of memory?!
		log.Printf("Cloning %s to memory...\n", url)
		return git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
			URL: url,
		})
	}
	log.Printf("Opening %s...\n", url)
	return git.PlainOpen(url)
}
