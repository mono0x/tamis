package plan

import "unicode/utf8"

// MaxNameBytes is the filename byte budget common filesystems enforce. Both the
// planner, which builds destination names, and the applier, which resolves
// conflicts into "name-2.ext", have to fit within it.
const MaxNameBytes = 255

// FitName trims base so that base+suffix stays within MaxNameBytes, never
// splitting a rune.
func FitName(base, suffix string) string {
	limit := MaxNameBytes - len(suffix)
	for len(base) > limit {
		_, size := utf8.DecodeLastRuneInString(base)
		base = base[:len(base)-size]
	}
	return base
}
