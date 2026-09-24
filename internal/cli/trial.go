// Package cli implements one-shot commands that run with the application
// configuration and exit before the HTTP server starts.
package cli

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/miebyte/goutils/flags"

	"github.com/superwhys/one-more-round/internal/domain/identity"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

const tokenLength = 64

var (
	trialOutput = flags.String(
		"trial-output",
		"",
		"write a one-use trial invitation to a private file and exit",
	)
	revokeTrial = flags.String(
		"revoke-trial-file",
		"",
		"revoke a trial invitation stored in the specified private file and exit",
	)
)

// Trials is the invitation persistence required by the trial commands.
type Trials interface {
	Create(ctx context.Context, hash string, expires time.Time) error
	Revoke(ctx context.Context, hash string) error
}

// RunTrial executes the trial command requested on the command line and reports
// whether it handled the invocation.
func RunTrial(ctx context.Context, store Trials, origin string) (bool, error) {
	return runTrial(ctx, store, origin, revokeTrial(), trialOutput())
}

// runTrial dispatches on the requested command paths; revoking wins when both
// flags are present.
func runTrial(
	ctx context.Context,
	store Trials,
	origin, revokePath, outputPath string,
) (bool, error) {
	switch {
	case revokePath != "":
		return true, RevokeTrial(ctx, store, revokePath)
	case outputPath != "":
		return true, IssueTrial(ctx, store, origin, outputPath)
	}
	return false, nil
}

// IssueTrial creates a one-use invitation valid for seven days and writes its
// link to a new file that only the current user can read.
func IssueTrial(ctx context.Context, store Trials, origin, path string) error {
	token := secure.NewID()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err = store.Create(
		ctx,
		secure.Hash(token),
		time.Now().UTC().Add(identity.TrialTTL),
	); err != nil {
		return err
	}
	_, err = file.WriteString(origin + "/login#trial=" + token + "\n")
	return err
}

// RevokeTrial consumes the unused invitation contained in the link file at path.
func RevokeTrial(ctx context.Context, store Trials, path string) error {
	token, err := invitationToken(path)
	if err != nil {
		return err
	}
	return store.Revoke(ctx, secure.Hash(token))
}

// invitationToken reads an invitation link file and returns its trial token.
func invitationToken(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(strings.TrimSpace(string(raw)))
	if err != nil {
		return "", err
	}
	values, err := url.ParseQuery(parsed.Fragment)
	if err != nil {
		return "", err
	}
	token := values.Get("trial")
	if len(token) != tokenLength {
		return "", errors.New("邀请文件无效：未包含有效的试用邀请链接")
	}
	return token, nil
}
