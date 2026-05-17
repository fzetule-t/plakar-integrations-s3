/*
 * Copyright (c) 2023 Gilles Chehade <gilles@poolp.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package exporter

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/PlakarKorp/kloset/connectors"
	"github.com/PlakarKorp/kloset/connectors/exporter"
	"github.com/PlakarKorp/kloset/location"
	"github.com/PlakarKorp/kloset/objects"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

type FSExporter struct {
	opts    *connectors.Options
	rootDir string

	hlCreate singleflight.Group // key -> ensures canonical exists, returns canonical abs path
	hlCanon  sync.Map           // key -> canonical abs path string
	hlMu     sync.Map           // key -> *sync.Mutex (serialize os.Link per key)
}

func init() {
	exporter.Register("fs", location.FLAG_LOCALFS, NewFSExporter)
}

func NewFSExporter(ctx context.Context, opts *connectors.Options, name string, config map[string]string) (exporter.Exporter, error) {
	location := config["location"]
	rootDir := strings.TrimPrefix(location, name+"://")

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	return &FSExporter{
		opts:    opts,
		rootDir: absRoot,
	}, nil
}

// isContained reports whether path is rooted within root (or is root itself).
// Both arguments must be clean absolute paths.
func isContained(root, path string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func (p *FSExporter) Root() string          { return p.rootDir }
func (p *FSExporter) Origin() string        { return p.opts.Hostname }
func (p *FSExporter) Type() string          { return "fs" }
func (p *FSExporter) Flags() location.Flags { return location.FLAG_LOCALFS }

func (p *FSExporter) Ping(ctx context.Context) error {
	return nil
}

func (p *FSExporter) SetPermissions(ctx context.Context, pathname string, fileinfo *objects.FileInfo) error {
	if fileinfo.Type() != "symlink" {
		if err := os.Chmod(pathname, fileinfo.Mode()); err != nil {
			return err
		}
	}
	if os.Geteuid() == 0 {
		if err := os.Lchown(pathname, int(fileinfo.Uid()), int(fileinfo.Gid())); err != nil {
			return err
		}
	}
	if fileinfo.Type() == "symlink" {
		if err := Lutimes(pathname, fileinfo.ModTime(), fileinfo.ModTime()); err != nil {
			return err
		}
	} else {
		if err := os.Chtimes(pathname, fileinfo.ModTime(), fileinfo.ModTime()); err != nil {
			return err
		}
	}
	return nil
}
