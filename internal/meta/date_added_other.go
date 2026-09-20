//go:build !darwin

package meta

import (
	"os"
	"time"
)

func readDateAdded(*os.File) (*time.Time, error) { return nil, nil }
