// codedocs: the deterministic half of the /codedocs skill, as a single executable.
//
// Everything here costs no tokens: inventory, manifest building, drift checks,
// change mapping, and serving the viewer (with the code panel). The viewer and
// its browser libraries are embedded, so the binary has no runtime dependencies
// apart from git (optional, for change tracking).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Set at build time: -ldflags "-X main.version=0.2.0"
var version = "dev"

const usage = `codedocs %s: tool for the /codedocs skill

Usage: codedocs <command> [options]   (run from the repo root)

  stats           [--depth N]                    Inventory of the repo (for /codedocs init)
  install-viewer  [FOLDER] [--project NAME] [--offline]
                                                 Write viewer files into the docs folder, create config
  manifest                                       Rebuild manifest.json from content/*.md
  check           [--json]                       Verify code refs, page links and coverage
  affected        [RANGE] [--json]               Which pages a change touches (default: since last sync)
  mark-synced                                    Record HEAD as the commit the docs describe
  status                                         Versions, sync point, outdated pages
  serve           [--port 8780] [--open]         Serve the viewer, with the code panel
  export          FOLDER [--github OWNER/REPO] [--ref COMMIT] [--source-url URL --web-url URL]
                                                 Static copy for any web host; the code panel loads
                                                 source from GitHub (default: origin, at the synced commit)
  find            TERM… [--json]                 Where a subject lives: folders, files and definitions
  links           [FILE] [--json]                Code-panel links: per-language summary, or one file's links
  stop            [--all]                        Stop this repo's viewer (or every running one)
  version         [--guidance]                   Print the tool version (or the page-guidance version)

Every command accepts --dir FOLDER to name the docs folder. Without it, the tool
looks for docs/docs.config.json, then docs/codebase/docs.config.json.
`

type options struct {
	dir       string
	project   string
	port      string
	depth     string
	json      bool
	guidance  bool
	offline   bool
	open      bool
	all       bool
	github    string
	ref       string
	sourceURL string
	webURL    string
	pos       []string
}

func parseArgs(args []string) options {
	o := options{port: "8780", depth: "3"}
	for i := 0; i < len(args); i++ {
		a := args[i]
		val := func() string {
			if i+1 >= len(args) {
				die("option " + a + " needs a value")
			}
			i++
			return args[i]
		}
		switch a {
		case "--dir":
			o.dir = val()
		case "--project":
			o.project = val()
		case "--port":
			o.port = val()
		case "--depth":
			o.depth = val()
		case "--json":
			o.json = true
		case "--offline":
			o.offline = true
		case "--open":
			o.open = true
		case "--guidance":
			o.guidance = true
		case "--all":
			o.all = true
		case "--github":
			o.github = val()
		case "--ref":
			o.ref = val()
		case "--source-url":
			o.sourceURL = val()
		case "--web-url":
			o.webURL = val()
		default:
			if strings.HasPrefix(a, "--") {
				die("unknown option " + a)
			}
			o.pos = append(o.pos, a)
		}
	}
	return o
}

var root string

func main() {
	if len(os.Args) < 2 || os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h" {
		fmt.Printf(usage, version)
		return
	}
	cmd := os.Args[1]
	o := parseArgs(os.Args[2:])
	var err error
	if root, err = os.Getwd(); err != nil {
		die(err.Error())
	}
	root, _ = filepath.Abs(root)
	switch cmd {
	case "stats":
		cmdStats(o)
	case "install-viewer":
		cmdInstallViewer(o)
	case "manifest":
		cmdManifest(o)
	case "check":
		cmdCheck(o)
	case "affected":
		cmdAffected(o)
	case "mark-synced":
		cmdMarkSynced(o)
	case "status":
		cmdStatus(o)
	case "serve":
		cmdServe(o)
	case "export":
		cmdExport(o)
	case "links":
		cmdLinks(o)
	case "find":
		cmdFind(o)
	case "stop", "shutdown":
		cmdStop(o)
	case "version", "--version":
		if o.guidance {
			fmt.Println(guidanceVersion())
		} else {
			fmt.Println(version)
		}
	default:
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}
}
