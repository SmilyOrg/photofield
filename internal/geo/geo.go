package geo

import (
	"context"
	"embed"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/golang/geo/s2"
	"github.com/smilyorg/tinygpkg/gpkg"
	"modernc.org/sqlite/vfs"
)

var ErrNotAvailable = fmt.Errorf("geopackage not available")

type Config struct {
	GeoPackage     GeoPackageConfig `json:"geopackage"`
	ReverseGeocode bool             `json:"reverse_geocode"`
}

type GeoPackageConfig struct {
	Path    string `json:"path"`
	Table   string `json:"table"`
	NameCol string `json:"name_col"`
}

type Geo struct {
	config Config
	uri    string
	fs     *vfs.FS
	gp     *gpkg.GeoPackage
}

// New creates a new Geo
//
// If reverse geocoding is enabled, it will attempt to open the geopackage
// using the `Path` in the config. If the path is empty, it will attempt to
// find the geopackage in the provided embed.FS.
//
// If reverse geocoding is disabled, it will return a Geo with no geopackage,
// leading to ErrNotAvailable being returned for all reverse geocoding requests.
//
// Call Close() on the returned Geo when you are done with it to free resources.
func New(config Config, fs embed.FS) (*Geo, error) {
	g := &Geo{
		config: config,
	}
	if !config.ReverseGeocode {
		return g, nil
	}
	if config.GeoPackage.Path == "" {
		// If no path is provided, find the geopackage in the embed.FS
		p, err := getGeoPackagePathFromFs(fs, ".")
		if err != nil {
			return nil, fmt.Errorf("error finding geopackage: %w", err)
		}
		if p == "" {
			return nil, fmt.Errorf("path not set and embedded geopackage not found")
		}
		n, f, err := vfs.New(fs)
		if err != nil {
			return nil, fmt.Errorf("error creating geopackage vfs: %w", err)
		}
		g.fs = f
		g.uri = "file:" + p + "?vfs=" + n + "&mode=ro"
	} else {
		// If a path is provided, use it
		g.uri = config.GeoPackage.Path
	}

	// Open the geopackage
	gp, err := gpkg.Open(
		g.uri,
		g.config.GeoPackage.Table,
		[]string{g.config.GeoPackage.NameCol},
	)
	if err != nil {
		g.Close()
		return nil, fmt.Errorf("error opening geopackage: %w", err)
	}
	g.gp = gp

	// Set up the geometry cache, this prevents having to re-parse the geometry
	// for every request
	c, err := NewCache()
	if err != nil {
		g.Close()
		return nil, fmt.Errorf("error creating geocache: %w", err)
	}
	g.gp.Cache = c
	return g, nil
}

func (g *Geo) Available() bool {
	return g != nil && g.config.ReverseGeocode && g.gp != nil
}

func (g *Geo) String() string {
	if g == nil || !g.config.ReverseGeocode {
		return "geo reverse geocoding disabled"
	}
	if g.gp == nil {
		return "geo geopackage not loaded"
	}
	return "geo using " + g.uri
}

// ReverseGeocode returns the name of the feature at the given location.
//
// If reverse geocoding is disabled, it will return ErrNotAvailable.
func (g *Geo) ReverseGeocode(ctx context.Context, l s2.LatLng) (string, error) {
	if !g.Available() {
		return "", ErrNotAvailable
	}
	cols, err := g.gp.ReverseGeocode(ctx, l)
	if err != nil {
		return "", fmt.Errorf("error reverse geocoding: %w", err)
	}
	return cols[0], nil
}

func (g *Geo) Close() error {
	if g == nil {
		return nil
	}
	if g.gp != nil {
		c, ok := g.gp.Cache.(*Cache)
		if !ok {
			return fmt.Errorf("error closing geopackage: cache is not a *Cache")
		}
		c.Close()
		err := g.gp.Close()
		if err != nil {
			return fmt.Errorf("error closing geopackage: %w", err)
		}
		g.gp = nil
	}
	if g.fs != nil {
		err := g.fs.Close()
		if err != nil {
			return fmt.Errorf("error closing geopackage vfs: %w", err)
		}
		g.fs = nil
	}
	return nil
}

func getGeoPackagePathFromFs(fs embed.FS, dir string) (path string, _ error) {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		log.Fatalf("failed to read geopackage vfs: %s", err)
	}
	for _, entry := range entries {
		n := entry.Name()
		if entry.IsDir() {
			p, err := getGeoPackagePathFromFs(fs, filepath.ToSlash(filepath.Join(dir, n)))
			if err != nil {
				return "", err
			}
			if p != "" {
				if path != "" {
					return "", fmt.Errorf("multiple geopackages found in %s", dir)
				}
				path = p
			}
			continue
		}
		if strings.HasSuffix(n, ".gpkg") {
			if path != "" {
				return "", fmt.Errorf("multiple geopackages found in %s", dir)
			}
			path = filepath.ToSlash(filepath.Join(dir, n))
		}
	}
	return path, nil
}
