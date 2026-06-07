// Package redact masks secrets in strings before they are surfaced to a
// user or a log. It targets credentials embedded in restic repository URLs
// (for example "s3:https://KEY:SECRET@host/bucket"), which would otherwise
// leak into command output, error messages, and the diagnostic log.
package redact

import "regexp"

// urlCredentials matches the password portion of URL userinfo
// ("scheme://user:password@host"), capturing everything up to and including
// the user and the colon so it can be preserved while the password is
// replaced. It only matches when a password is present (a colon before "@").
var urlCredentials = regexp.MustCompile(`(://[^/@:\s]+:)[^@/\s]+@`)

// redactedPassword is the placeholder substituted for a URL password.
const redactedPassword = "***"

// Credentials returns s with any URL password masked. The scheme, user, and
// host are preserved so the value stays recognizable; only the secret is
// removed. Strings without embedded URL credentials are returned unchanged.
func Credentials(s string) string {
	return urlCredentials.ReplaceAllString(s, "${1}"+redactedPassword+"@")
}
