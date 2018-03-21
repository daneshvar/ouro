package rtsp

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrAuthMalformedChallenge indicates incorrectly formatted WWW-Authenticate header.
	ErrAuthMalformedChallenge = errors.New("Malformed authentication challenge header")
	// ErrAuthNotImpemented indicates missing implementation for authentication method or encryption.
	ErrAuthNotImpemented = errors.New("Missing implementation for authentication method")
)

// DigestAuth is function to call to encoude authorization for a particular verb in session.
type DigestAuth func(verb string) string

// Digest encapsulates all information necessary to perform digest authentication against a remote site.
type Digest struct {
	basic     bool
	username  string
	realm     string
	domain    string
	nonce     string
	opaque    string
	stale     string
	algorithm string
	cnonce    string
	qop       string
	uri       string
	ha1       string
	ha2       string
	scount    string
	count     int
}

// NewDigest parses challenge and creates a new basic/digest authentication processor.
func NewDigest(uri, challenge string) (*Digest, error) {
	d := &Digest{count: 0, uri: uri}
	d.algorithm = "MD5"
	challenge = strings.TrimSpace(challenge)
	if strings.HasPrefix(challenge, "Digest ") {
		d.basic = false
		challenge = challenge[7:]
	} else if strings.HasPrefix(challenge, "Basic ") {
		d.basic = true
		challenge = challenge[6:]
	} else {
		return nil, ErrAuthMalformedChallenge
	}
	parts := strings.Split(challenge, ",")
	for i := 0; i < len(parts); i++ {
		parts[i] = strings.TrimSpace(parts[i])
		if len(parts[i]) == 0 {
			continue
		}

		attr, val := parts[i], ""
		if j := strings.Index(attr, "="); j > 0 {
			attr, val = strings.ToLower(attr[:j]), attr[j+1:]
			val = strings.Trim(val, "\"")
			switch attr {
			case "realm":
				d.realm = val
			case "domain":
				d.domain = val
			case "nonce":
				d.nonce = val
			case "opaque":
				d.opaque = val
			case "stale":
				d.stale = val
			case "algorithm":
				d.algorithm = val
			case "qop":
				vals := strings.Split(val, ",")
				if len(vals) > 0 {
					d.qop = strings.ToLower(vals[0])
				}
			default:
				return nil, ErrAuthMalformedChallenge
			}
		}
	}
	if d.nonce != "" {
		// FIXME: Make algorithm comparison case-insensitive.
		if d.algorithm != "MD5" && d.algorithm != "MD5-sess" {
			return nil, ErrAuthNotImpemented
		}
	}
	return d, nil
}

// Authenticate creates value for Authorization header.
func (d *Digest) Authenticate(username, password string) DigestAuth {
	out := strings.Builder{}
	if d.basic {
		out.WriteString("Basic ")
		out.WriteString(base64.StdEncoding.EncodeToString([]byte(colonnade(username, password))))
		auth := out.String()
		return func(verb string) string {
			return auth
		}
	}
	d.username = username
	d.ha1 = md5hex(colonnade(username, d.realm, password))
	out.WriteString("Digest username=\"")
	out.WriteString(d.username)
	out.WriteString("\", realm=\"")
	out.WriteString(d.realm)
	out.WriteString("\", nonce=\"")
	out.WriteString(d.nonce)
	out.WriteString("\", uri=\"")
	out.WriteString(d.uri)
	out.WriteString("\", algorithm=\"")
	out.WriteString(d.algorithm)
	if d.opaque != "" {
		out.WriteString("\", opaque=\"")
		out.WriteString(d.opaque)
	}
	if d.qop != "" {
		d.next()
		// FIXME: Make algorithm comparison case-insensitive.
		if d.algorithm == "MD5-sess" {
			d.ha1 = md5hex(colonnade(d.ha1, d.nonce, d.cnonce))
		}
		out.WriteString("\", qop=")
		out.WriteString(d.qop)
		out.WriteString(", nc=")
		out.WriteString(d.scount)
		out.WriteString(", cnonce=\"")
		out.WriteString(d.cnonce)
	}
	out.WriteString("\"")
	auth := out.String()
	return func(verb string) string {
		return auth + d.response(verb)
	}
}

func (d *Digest) next() {
	d.count++
	d.scount = fmt.Sprintf("%08x", d.count)
	if d.qop == "auth" {
		b := make([]byte, 8)
		rand.Read(b)
		d.cnonce = hex.EncodeToString(b)[:16]
	}
}

func (d *Digest) response(verb string) string {
	if d.qop == "auth-int" {
		body := "" // FIXME: Pass message body to this method
		d.ha2 = md5hex(colonnade(verb, d.uri, md5hex(body)))
	} else {
		d.ha2 = md5hex(colonnade(verb, d.uri))
	}
	if d.qop == "" {
		return ", response=\"" + md5hex(colonnade(d.ha1, d.nonce, d.ha2)) + "\""
	}
	return ", response=\"" + md5hex(colonnade(d.ha1, d.nonce, d.scount, d.cnonce, d.qop, d.ha2)) + "\""
}

func md5hex(data string) string {
	hf := md5.New()
	hf.Write([]byte(data))
	return hex.EncodeToString(hf.Sum(nil))
}

func colonnade(params ...string) string {
	return strings.Join(params, ":")
}