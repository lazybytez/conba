package redact_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/support/redact"
)

func TestCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "s3 url with key and secret",
			in:   "s3:https://AKIAKEY:wJalrSecret@s3.amazonaws.com/bucket",
			want: "s3:https://AKIAKEY:***@s3.amazonaws.com/bucket",
		},
		{
			name: "rest url with user and password and port",
			in:   "rest:https://user:p4ssw0rd@host:8000/repo",
			want: "rest:https://user:***@host:8000/repo",
		},
		{
			name: "local path unchanged",
			in:   "/var/backups/restic",
			want: "/var/backups/restic",
		},
		{
			name: "b2 backend without url creds unchanged",
			in:   "b2:bucketname:path/to/repo",
			want: "b2:bucketname:path/to/repo",
		},
		{
			name: "url without userinfo unchanged",
			in:   "s3:http://minio:9000/bucket",
			want: "s3:http://minio:9000/bucket",
		},
		{
			name: "user without password unchanged",
			in:   "sftp:user@host:/srv/restic",
			want: "sftp:user@host:/srv/restic",
		},
		{
			name: "credentials embedded in surrounding text",
			in:   "Fatal: unable to open repo at rest:https://u:topsecret@h/r: denied",
			want: "Fatal: unable to open repo at rest:https://u:***@h/r: denied",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := redact.Credentials(test.in)
			if got != test.want {
				t.Errorf("Credentials(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}
