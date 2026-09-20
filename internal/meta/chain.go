package meta

import (
	"context"
	"fmt"
	"io"
	"time"
)

type Chain struct {
	Providers      []Provider
	RequiredFields func(string) Fields
}

// NewChain builds the provider chain in priority order: specialized image and
// video parsers first, sidecars next, and the filesystem as the last resort.
func NewChain(assume, mvhd *time.Location) Chain {
	return Chain{Providers: []Provider{
		ImageProvider{AssumeLocation: assume},
		ImageExtProvider{AssumeLocation: assume},
		PNGTextProvider{AssumeLocation: assume},
		VideoProvider{MVHDLocation: mvhd},
		VideoSidecarProvider{AssumeLocation: assume},
		FSProvider{},
	}}
}

// Extract runs the chain under panic recovery, so neither a provider nor the
// format sniffing that precedes it can crash a scan worker.
func (c Chain) Extract(ctx context.Context, path string) (*Record, error) {
	return safeExtract(func() (*Record, error) { return c.extract(ctx, path) })
}

func (c Chain) extract(ctx context.Context, path string) (*Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var fields Fields
	if c.RequiredFields != nil {
		fields = c.RequiredFields(path)
	}
	file, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	base, err := newRecord(file, path, fields)
	if err != nil {
		return nil, err
	}
	var warnings []string
	var best *Record
	specializedFailed := false
	for _, p := range c.Providers {
		if !fields.HasAny(p.Provides()...) || !p.Supports(base.Format) {
			continue
		}
		r, err := safeExtract(func() (*Record, error) {
			reader := contextReader{ctx, file}
			if _, err := reader.Seek(0, io.SeekStart); err != nil {
				return nil, err
			}
			return p.Extract(ctx, base, file)
		})
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		if err != nil {
			warnings = append(warnings, err.Error())
			if _, fallback := p.(FSProvider); !fallback {
				specializedFailed = true
			}
			continue
		}
		if best == nil {
			best = r
		} else {
			if best.Captured == nil {
				best.Captured = r.Captured
			}
			best.Warnings = append(best.Warnings, r.Warnings...)
			for k, v := range r.Raw {
				if _, exists := best.Raw[k]; !exists {
					best.Raw[k] = v
				}
			}
		}
		if best.Captured != nil || !fields.captured() {
			break
		}
	}
	if best == nil && !fields.captured() {
		best = base
	}
	if best == nil {
		return nil, fmt.Errorf("no provider extracted %s: %v", path, warnings)
	}
	best.ExtractionFailed = specializedFailed
	best.Warnings = append(warnings, best.Warnings...)
	return best, nil
}

// safeExtract recovers third-party parser panics into errors. Every call into a
// provider goes through it, so panic containment lives in one place.
func safeExtract(extract func() (*Record, error)) (record *Record, err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("provider panic: %v", v)
		}
	}()
	return extract()
}
