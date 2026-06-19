package cli

import (
	"errors"

	"github.com/tamnd/arstechnica-cli/arstechnica"
)

func isNotFound(err error) bool {
	return errors.Is(err, arstechnica.ErrUnknownSection)
}

func mapFetchErr(err error) error {
	if err == nil {
		return nil
	}
	if isNotFound(err) {
		return codeError(exitUsage, err)
	}
	return codeError(exitError, err)
}
