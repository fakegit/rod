package main

import (
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/ysmood/kit"
)

var (
	// MirrorChromium to fetch the latest chromium version
	MirrorChromium = "https://npm.taobao.org/mirrors/chromium-browser-snapshots/Linux_x64/"
	// MirrorChromiumRegExp to match the MirrorChromium html source
	MirrorChromiumRegExp = regexp.MustCompile(`\Q"/mirrors/chromium-browser-snapshots/Linux_x64/\E(\d+)`)
)

var slash = filepath.FromSlash

func main() {
	s, err := kit.Req(MirrorChromium).String()
	kit.E(err)

	matchs := MirrorChromiumRegExp.FindAllStringSubmatch(s, -1)
	if len(matchs) <= 0 {
		kit.E(fmt.Errorf("cannot match version of the latest chromium from %s", MirrorChromium))
	}

	revision := matchs[len(matchs)-1][1]

	if revision == "" {
		kit.E(fmt.Errorf("empty version of the latest chromium %s", revision))
	}

	build := kit.S(`// generated by running "go generate" on project root

package launcher

// DefaultRevision for chrome
// curl -s -S https://www.googleapis.com/download/storage/v1/b/chromium-browser-snapshots/o/Mac%2FLAST_CHANGE\?alt\=media
const DefaultRevision = {{.revision}}
`,
		"revision", revision,
	)

	kit.E(kit.OutputFile(slash("lib/launcher/revision.go"), build, nil))

}
