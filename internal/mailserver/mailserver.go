// Package mailserver handles outbound email delivery.
package mailserver

import (
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"

	"github.com/emersion/go-smtp"
)

type MailServer interface {
	Start() error
	AddHook(hook Hook)

	NewSession(c *smtp.Conn) (smtp.Session, error)
}

type Impl struct {
	srv     *smtp.Server
	hooks   []Hook
	queries *dbgen.Queries
	queue   queue.JobQueue
}

func NewMailServer(addr, domain string, queries *dbgen.Queries, q queue.JobQueue) MailServer {
	backend := &Impl{queries: queries, queue: q}
	backend.srv = smtp.NewServer(backend)

	backend.srv.Addr = addr
	backend.srv.Domain = domain
	backend.srv.AllowInsecureAuth = true

	return backend
}

func (backend *Impl) Start() error {
	err := backend.srv.ListenAndServe()
	return err
}

// NewSession is called after client greeting (EHLO, HELO).
func (backend *Impl) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &Session{
		hooks:   backend.hooks,
		queries: backend.queries,
		queue:   backend.queue,
	}, nil
}

// AddHook adds a new hook to the mail server.
func (backend *Impl) AddHook(hook Hook) {
	backend.hooks = append(backend.hooks, hook)
}
