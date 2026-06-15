// Package analytics reports Touchline usage to PostHog. It is intentionally
// best-effort and optional: a nil *Tracker is a no-op, so analytics can be left
// disabled simply by not configuring an API key.
//
// Both local runs and SSH sessions emit the same "session_started" /
// "session_ended" events (distinguished by a "mode" property), so PostHog's
// built-in unique-user and DAU/WAU charts cover every way the app is used.
package analytics

import (
	"net"
	"os"
	"os/user"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/posthog/posthog-go"
	gossh "golang.org/x/crypto/ssh"
)

const defaultHost = "https://us.i.posthog.com"

const (
	eventStarted = "session_started"
	eventEnded   = "session_ended"
)

type Tracker struct {
	client posthog.Client
	active int64
}

// New returns a Tracker, or nil (disabled) when apiKey is empty.
func New(apiKey, host string) (*Tracker, error) {
	if apiKey == "" {
		return nil, nil
	}
	if host == "" {
		host = defaultHost
	}

	client, err := posthog.NewWithConfig(apiKey, posthog.Config{Endpoint: host})
	if err != nil {
		return nil, err
	}
	return &Tracker{client: client}, nil
}

// Close flushes any buffered events. Safe to call on a nil Tracker.
func (t *Tracker) Close() {
	if t == nil || t.client == nil {
		return
	}
	_ = t.client.Close()
}

// AppStarted records the start of a local (non-SSH) usage session.
func (t *Tracker) AppStarted() {
	if t == nil || t.client == nil {
		return
	}
	t.enqueue(localDistinctID(), eventStarted, posthog.NewProperties().
		Set("mode", "local"))
}

// AppEnded records the end of a local usage session, including its duration.
func (t *Tracker) AppEnded(duration time.Duration) {
	if t == nil || t.client == nil {
		return
	}
	t.enqueue(localDistinctID(), eventEnded, posthog.NewProperties().
		Set("mode", "local").
		Set("duration_seconds", duration.Seconds()))
}

// Connected records the start of an SSH session and returns the new active count.
func (t *Tracker) Connected(sess ssh.Session) int64 {
	if t == nil || t.client == nil {
		return 0
	}
	active := atomic.AddInt64(&t.active, 1)
	t.enqueue(distinctID(sess), eventStarted, sshProps(sess).
		Set("active_sessions", active))
	return active
}

// Disconnected records the end of an SSH session (with its duration) and returns
// the new active count.
func (t *Tracker) Disconnected(sess ssh.Session, duration time.Duration) int64 {
	if t == nil || t.client == nil {
		return 0
	}
	active := atomic.AddInt64(&t.active, -1)
	if active < 0 {
		active = 0
	}
	t.enqueue(distinctID(sess), eventEnded, sshProps(sess).
		Set("active_sessions", active).
		Set("duration_seconds", duration.Seconds()))
	return active
}

func (t *Tracker) enqueue(distinctID, event string, props posthog.Properties) {
	_ = t.client.Enqueue(posthog.Capture{
		DistinctId: distinctID,
		Event:      event,
		Properties: props,
	})
}

func sshProps(sess ssh.Session) posthog.Properties {
	pty, _, _ := sess.Pty()
	return posthog.NewProperties().
		Set("mode", "ssh").
		Set("term", pty.Term).
		Set("remote_ip", remoteIP(sess)).
		Set("ssh_user", sess.User())
}

// distinctID identifies an SSH visitor by their public key fingerprint when
// available (stable across IP changes), otherwise by their IP address.
func distinctID(sess ssh.Session) string {
	if pk := sess.PublicKey(); pk != nil {
		return gossh.FingerprintSHA256(pk)
	}
	return remoteIP(sess)
}

func remoteIP(sess ssh.Session) string {
	addr := sess.RemoteAddr().String()
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

// localDistinctID identifies a local run by the developer's machine
// (user@hostname), so repeated dev sessions roll up to one person in PostHog.
func localDistinctID() string {
	name := "unknown"
	if u, err := user.Current(); err == nil && u.Username != "" {
		name = u.Username
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		return name + "@" + host
	}
	return name
}
