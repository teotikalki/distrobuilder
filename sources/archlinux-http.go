package sources

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lxc/distrobuilder/shared"

	lxd "github.com/lxc/lxd/shared"
	"gopkg.in/antchfx/htmlquery.v1"
)

// ArchLinuxHTTP represents the Arch Linux downloader.
type ArchLinuxHTTP struct{}

// NewArchLinuxHTTP creates a new ArchLinuxHTTP instance.
func NewArchLinuxHTTP() *ArchLinuxHTTP {
	return &ArchLinuxHTTP{}
}

// Run downloads an Arch Linux tarball.
func (s *ArchLinuxHTTP) Run(definition shared.Definition, rootfsDir string) error {
	release := definition.Image.Release

	if release == "" {
		var err error

		// Get latest release
		release, err = s.getLatestRelease()
		if err != nil {
			return err
		}
	}

	fname := fmt.Sprintf("archlinux-bootstrap-%s-%s.tar.gz",
		release, definition.Image.ArchitectureMapped)
	tarball := fmt.Sprintf("%s/%s/%s", definition.Source.URL,
		release, fname)

	url, err := url.Parse(tarball)
	if err != nil {
		return err
	}

	if !definition.Source.SkipVerification && url.Scheme != "https" &&
		len(definition.Source.Keys) == 0 {
		return errors.New("GPG keys are required if downloading from HTTP")
	}

	err = shared.DownloadHash(tarball, "", nil)
	if err != nil {
		return err
	}

	// Force gpg checks when using http
	if !definition.Source.SkipVerification && url.Scheme != "https" {
		shared.DownloadHash(tarball+".sig", "", nil)

		valid, err := shared.VerifyFile(
			filepath.Join(os.TempDir(), fname),
			filepath.Join(os.TempDir(), fname+".sig"),
			definition.Source.Keys,
			definition.Source.Keyserver)
		if err != nil {
			return err
		}
		if !valid {
			return errors.New("Failed to verify tarball")
		}
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(os.TempDir(), fname), rootfsDir, false, false, nil)
	if err != nil {
		return err
	}

	// Move everything inside 'root.x86_64' (which was is the tarball) to its
	// parent directory
	files, err := filepath.Glob(fmt.Sprintf("%s/*", filepath.Join(rootfsDir,
		"root."+definition.Image.ArchitectureMapped)))
	if err != nil {
		return err
	}

	for _, file := range files {
		err = os.Rename(file, filepath.Join(rootfsDir, path.Base(file)))
		if err != nil {
			return err
		}
	}

	return os.RemoveAll(filepath.Join(rootfsDir, "root."+
		definition.Image.ArchitectureMapped))
}

func (s *ArchLinuxHTTP) getLatestRelease() (string, error) {
	doc, err := htmlquery.LoadURL("https://www.archlinux.org/download/")
	if err != nil {
		return "", err
	}

	node := htmlquery.FindOne(doc, `//*[@id="arch-downloads"]/ul[1]/li[1]/text()`)
	if node == nil {
		return "", fmt.Errorf("Failed to determine latest release")
	}

	return strings.TrimSpace(node.Data), nil
}
